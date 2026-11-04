
FROM busybox
RUN cat /etc/hosts | grep myhost | grep 1.2.3.4
RUN cat /etc/hosts | grep myhostmulti | grep 162.242.195.81
RUN cat /etc/hosts | grep myhostmulti | grep 162.242.195.82
