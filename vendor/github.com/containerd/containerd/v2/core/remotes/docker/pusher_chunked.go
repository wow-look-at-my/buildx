/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/v2/core/content"
)

// defaultPushChunkSize is the recommended chunk size for chunked blob uploads.
// 90 MiB sits safely below the 100 MB request-body limit imposed by common
// reverse proxies (for example Cloudflare's edge cap), so no single PATCH
// request exceeds it.
const defaultPushChunkSize = 90 << 20

// pushChunkSizeEnv is the environment variable that configures chunked uploads.
const pushChunkSizeEnv = "CONTAINERD_PUSH_CHUNK_SIZE"

// pushChunkSize returns the chunk size (in bytes) to use for blob uploads.
//
// Chunked upload is opt-in and disabled by default: when
// CONTAINERD_PUSH_CHUNK_SIZE is unset (or 0) the pusher keeps the original
// monolithic single-PUT behaviour. A positive value enables the
// OCI/Distribution PATCH chunk flow with that chunk size. The special value
// "default" selects defaultPushChunkSize (90 MiB). A misconfigured value falls
// back to the monolithic path rather than risking one unexpectedly huge PUT.
func pushChunkSize() int64 {
	v := strings.TrimSpace(os.Getenv(pushChunkSizeEnv))
	switch strings.ToLower(v) {
	case "":
		return 0
	case "default":
		return defaultPushChunkSize
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// chunkedPushWriter is a content.Writer that uploads a blob using the
// OCI/Distribution chunked upload protocol: a sequence of PATCH requests (each
// carrying at most chunkSize bytes with a Content-Range header) followed by a
// final PUT that commits the blob by digest. Keeping every individual HTTP
// request below chunkSize is what allows the upload to traverse reverse proxies
// that enforce a request-body size limit.
//
// The blob-upload session (POST .../blobs/uploads/), the cross-repo mount and
// the already-exists short-circuit are all handled by dockerPusher.push before
// this writer is constructed, so they behave identically to the monolithic
// path.
type chunkedPushWriter struct {
	base    *dockerBase
	tracker StatusTracker

	ref      string
	expected digest.Digest
	hostname string // refspec hostname, used for addNamespace

	ctx context.Context

	host     RegistryHost
	location *url.URL

	chunkSize int64

	buf       bytes.Buffer
	offset    int64 // number of bytes already accepted by the registry
	committed bool
	err       error
}

func newChunkedPushWriter(ctx context.Context, base *dockerBase, host RegistryHost, location *url.URL, ref string, desc ocispec.Descriptor, tracker StatusTracker, hostname string, chunkSize int64, respHeader http.Header) *chunkedPushWriter {
	// Honour the minimum chunk length advertised by the registry on the upload
	// session: sending chunks smaller than this would be rejected.
	if v := strings.TrimSpace(respHeader.Get("OCI-Chunk-Min-Length")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > chunkSize {
			chunkSize = n
		}
	}
	return &chunkedPushWriter{
		base:      base,
		tracker:   tracker,
		ref:       ref,
		expected:  desc.Digest,
		hostname:  hostname,
		ctx:       ctx,
		host:      host,
		location:  location,
		chunkSize: chunkSize,
	}
}

func (cw *chunkedPushWriter) Write(p []byte) (int, error) {
	if cw.err != nil {
		return 0, cw.err
	}
	cw.buf.Write(p)
	for int64(cw.buf.Len()) >= cw.chunkSize {
		chunk := cw.buf.Next(int(cw.chunkSize))
		if err := cw.flushChunk(chunk); err != nil {
			cw.err = err
			return len(p), err
		}
	}
	cw.updateStatus()
	return len(p), nil
}

// flushChunk uploads chunk as a single PATCH request beginning at the current
// offset and advances offset by the number of accepted bytes.
func (cw *chunkedPushWriter) flushChunk(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	start := cw.offset
	end := cw.offset + int64(len(chunk)) - 1

	req, err := cw.newRequest(http.MethodPatch)
	if err != nil {
		return err
	}
	req.header.Set("Content-Type", "application/octet-stream")
	req.header.Set("Content-Range", fmt.Sprintf("%d-%d", start, end))
	req.body = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(chunk)), nil
	}
	req.size = int64(len(chunk))

	resp, err := req.doWithRetries(cw.ctx, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		// chunk accepted
	case http.StatusRequestedRangeNotSatisfiable:
		// 416: our range did not line up with what the registry already holds.
		// Resync to the registry's accepted range and resend the unaccepted tail.
		return cw.resumeFromRange(resp, chunk, start)
	default:
		return unexpectedResponseErr(resp)
	}

	// Advance the offset, preferring the registry-reported Range when present.
	cw.offset = end + 1
	if r := resp.Header.Get("Range"); r != "" {
		if rangeEnd, ok := parseRangeEnd(r); ok {
			cw.offset = rangeEnd + 1
		}
	}

	// Each PATCH may hand back a new upload location.
	if loc := resp.Header.Get("Location"); loc != "" {
		if err := cw.setLocation(loc, resp); err != nil {
			return err
		}
	}
	return nil
}

// resumeFromRange handles a 416 response by reconciling our offset with the
// range the registry reports it already holds, then resending only the tail of
// the current chunk that has not yet been accepted.
func (cw *chunkedPushWriter) resumeFromRange(resp *http.Response, chunk []byte, start int64) error {
	r := resp.Header.Get("Range")
	if r == "" {
		return fmt.Errorf("registry returned 416 without a Range header: %w", errdefs.ErrUnavailable)
	}
	end, ok := parseRangeEnd(r)
	if !ok {
		return fmt.Errorf("registry returned 416 with unparsable Range %q: %w", r, errdefs.ErrInvalidArgument)
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		if err := cw.setLocation(loc, resp); err != nil {
			return err
		}
	}

	accepted := end + 1            // total bytes the registry already holds
	chunkEnd := start + int64(len(chunk)) // exclusive end of this chunk
	switch {
	case accepted >= chunkEnd:
		// The registry already has this entire chunk; nothing more to send.
		cw.offset = accepted
		return nil
	case accepted <= start:
		// The registry is behind where this chunk began; we cannot reconcile.
		return fmt.Errorf("registry range %q precedes chunk start %d: %w", r, start, errdefs.ErrUnavailable)
	default:
		// Resend only the portion of this chunk the registry has not accepted.
		cw.offset = accepted
		return cw.flushChunk(chunk[accepted-start:])
	}
}

func (cw *chunkedPushWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	if cw.err != nil {
		return cw.err
	}

	// Flush any remaining buffered bytes as a final chunk.
	if cw.buf.Len() > 0 {
		chunk := cw.buf.Next(cw.buf.Len())
		if err := cw.flushChunk(chunk); err != nil {
			cw.err = err
			return err
		}
	}

	status, err := cw.tracker.GetStatus(cw.ref)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if size > 0 && size != cw.offset {
		return fmt.Errorf("unexpected size %d, expected %d", cw.offset, size)
	}

	if expected == "" {
		expected = status.Expected
	} else if expected != status.Expected {
		return fmt.Errorf("unexpected digest received: got %q, expected %q", status.Expected, expected)
	}

	// The final PUT commits the blob by digest with an empty body.
	loc := *cw.location
	q := loc.Query()
	q.Set("digest", expected.String())
	loc.RawQuery = q.Encode()

	req, err := cw.requestForLocation(http.MethodPut, &loc)
	if err != nil {
		return err
	}
	req.header.Set("Content-Type", "application/octet-stream")
	req.size = 0

	resp, err := req.doWithRetries(ctx, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 201 is the specified return status; some registries return 200/202/204.
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusAccepted:
	default:
		return unexpectedResponseErr(resp)
	}

	if dgstHdr := resp.Header.Get("Docker-Content-Digest"); dgstHdr != "" {
		actual, err := digest.Parse(dgstHdr)
		if err != nil {
			return fmt.Errorf("invalid content digest in response: %w", err)
		}
		if actual != expected {
			return fmt.Errorf("got digest %s, expected %s", actual, expected)
		}
	} else {
		log.G(ctx).Info("registry did not send a Docker-Content-Digest header")
	}

	status.Committed = true
	status.Offset = cw.offset
	status.UpdatedAt = time.Now()
	cw.tracker.SetStatus(cw.ref, status)
	cw.committed = true
	return nil
}

func (cw *chunkedPushWriter) Close() error {
	status, err := cw.tracker.GetStatus(cw.ref)
	if err == nil && !status.Committed {
		// Closing an incomplete writer. Record this as an error so that a
		// following write can retry it.
		status.ErrClosed = errors.New("closed incomplete writer")
		status.UpdatedAt = time.Now()
		cw.tracker.SetStatus(cw.ref, status)
	}
	return nil
}

func (cw *chunkedPushWriter) Status() (content.Status, error) {
	status, err := cw.tracker.GetStatus(cw.ref)
	if err != nil {
		return content.Status{}, err
	}
	return status.Status, nil
}

func (cw *chunkedPushWriter) Digest() digest.Digest {
	return cw.expected
}

func (cw *chunkedPushWriter) Truncate(size int64) error {
	return errors.New("cannot truncate remote upload")
}

func (cw *chunkedPushWriter) updateStatus() {
	status, err := cw.tracker.GetStatus(cw.ref)
	if err != nil {
		return
	}
	status.Offset = cw.offset + int64(cw.buf.Len())
	status.UpdatedAt = time.Now()
	cw.tracker.SetStatus(cw.ref, status)
}

// newRequest builds a request targeting the current upload location.
func (cw *chunkedPushWriter) newRequest(method string) (*request, error) {
	return cw.requestForLocation(method, cw.location)
}

func (cw *chunkedPushWriter) requestForLocation(method string, loc *url.URL) (*request, error) {
	req := cw.base.request(cw.host, method)
	req.path = loc.EscapedPath()
	if loc.RawQuery != "" {
		req.path += "?" + loc.RawQuery
	}
	if err := req.addNamespace(cw.hostname); err != nil {
		return nil, err
	}
	return req, nil
}

// setLocation updates the upload location from a registry response, resolving
// relative locations against the current one and following the same
// host/scheme-change handling as the monolithic push path.
func (cw *chunkedPushWriter) setLocation(location string, resp *http.Response) error {
	ref, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("failed to parse upload location %q: %w", location, err)
	}
	resolved := cw.location.ResolveReference(ref)
	if resolved.Host != cw.host.Host || resolved.Scheme != cw.host.Scheme {
		// The upload was redirected to a different host (e.g. a blob store/CDN).
		requested := resp.Request.URL
		if requested.Host != resolved.Host || requested.Scheme != resolved.Scheme {
			// Strip the authorizer when the host or scheme changes.
			cw.host.Authorizer = nil
			log.G(cw.ctx).WithField("host", resolved.Host).WithField("scheme", resolved.Scheme).Debug("chunked upload changed destination, authorizer removed")
		}
		cw.host.Host = resolved.Host
		cw.host.Scheme = resolved.Scheme
	}
	cw.location = resolved
	return nil
}

// parseRangeEnd parses the inclusive end byte from a registry upload Range
// header. The OCI/Distribution form is "<start>-<end>" (e.g. "0-1023"); some
// registries prefix it with "bytes=" or "bytes " or append a total
// ("0-1023/4096").
func parseRangeEnd(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "bytes=")
	s = strings.TrimPrefix(s, "bytes ")
	_, endStr, ok := strings.Cut(s, "-")
	if !ok {
		return 0, false
	}
	endStr = strings.TrimSpace(endStr)
	if i := strings.IndexByte(endStr, '/'); i >= 0 {
		endStr = strings.TrimSpace(endStr[:i])
	}
	n, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
