# Use official Ubuntu image as base image
FROM golang:1.24.4

# Set non-interactive installation environment variable
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y libopus-dev libopusfile-dev pkg-config

RUN apt-get install -y openssh-server

RUN apt-get install -y git tree || true
RUN apt-get install -y net-tools telnet netcat || true
RUN apt-get install -y unzip vim iptables iputils-ping || true

COPY docker/lib/onnxruntime-linux-x64-1.21.0.tgz /tmp/

RUN cd /tmp && \
    tar -xzf onnxruntime-linux-x64-1.21.0.tgz && \
    mkdir -p /usr/local/include/onnxruntime && \
    cp -r onnxruntime-linux-x64-1.21.0/include/* /usr/local/include/onnxruntime/ && \
    cp -r onnxruntime-linux-x64-1.21.0/lib/* /usr/local/lib/ && \
    rm -rf onnxruntime-linux-x64-1.21.0* && \
    ldconfig

ENV ONNXRUNTIME_DIR=/usr/local
ENV CGO_CFLAGS="-I${ONNXRUNTIME_DIR}/include/onnxruntime" 
ENV CGO_LDFLAGS="-L${ONNXRUNTIME_DIR}/lib -lonnxruntime"


ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=1

# Add environment variables to .bashrc and .profile to ensure all shells can access them
RUN echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc && \
    echo 'export GOPATH=/go' >> /root/.bashrc && \
    echo 'export GOROOT=/usr/local/go' >> /root/.bashrc && \
    echo 'export GOPROXY=https://goproxy.cn,direct' >> /root/.bashrc && \
    echo 'export CGO_ENABLED=1' >> /root/.bashrc && \
    echo 'export ONNXRUNTIME_DIR=/usr/local' >> /root/.bashrc && \
    echo 'export CGO_CFLAGS="-I${ONNXRUNTIME_DIR}/include/onnxruntime"' >> /root/.bashrc && \
    echo 'export CGO_LDFLAGS="-L${ONNXRUNTIME_DIR}/lib -lonnxruntime"' >> /root/.bashrc && \
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile && \
    echo 'export GOPATH=/go' >> /etc/profile && \
    echo 'export GOROOT=/usr/local/go' >> /etc/profile && \
    echo 'export GOPROXY=https://goproxy.cn,direct' >> /etc/profile && \
    echo 'export CGO_ENABLED=1' >> /etc/profile && \
    echo 'export ONNXRUNTIME_DIR=/usr/local' >> /etc/profile && \
    echo 'export CGO_CFLAGS="-I${ONNXRUNTIME_DIR}/include/onnxruntime"' >> /etc/profile && \
    echo 'export CGO_LDFLAGS="-L${ONNXRUNTIME_DIR}/lib -lonnxruntime"' >> /etc/profile

# Create Go directory structure
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" "$GOPATH/pkg" && chmod -R 777 "$GOPATH"

# Verify Go installation
RUN /usr/local/go/bin/go version || echo "Go installation failed, please check logs"

# Create working directory
RUN mkdir -p /workspace
WORKDIR /workspace

# Create SSH service runtime directory
RUN mkdir -p /var/run/sshd

# Set root user password
ARG PASSWORD=rootpassword
RUN echo "root:$PASSWORD" | chpasswd

# Configure SSH to allow root login
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config || true
RUN sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd || true

# Set timezone to Shanghai
RUN ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime || true

# Copy startup script
#COPY start-service.sh /usr/local/bin/
#RUN chmod +x /usr/local/bin/start-service.sh

#COPY . .
#RUN go build -o /workspace/xiaozhi_server /workspace/xiaozhi/cmd/server/  # 编译生成二进制文件

# Create startup script
RUN echo '#!/bin/bash' > /start.sh && \
    echo 'service ssh start' >> /start.sh && \
    echo 'cd /workspace' >> /start.sh && \
    echo 'exec bash' >> /start.sh && \
    chmod +x /start.sh

# Expose SSH port and application port
EXPOSE 22 8080

# Start SSH and keep container running
CMD ["/start.sh"]

# docker run -it --privileged --name ubt -d -p 9022:22 liuwang68/ubtool:1.0
# docker commit ubt zipdiff:1.0   
# docker run -it --privileged --name ubt -d -p 9022:22 zipdiff:1.0

# docker build --build-arg PASSWORD=root1234 -t liuwang68/ubtool:1.0 . -f DockerfileBase
# docker build --build-arg PASSWORD=root1234 -t ub:1.0 . -f DockerfileUbuntu
# docker run -it --privileged --name ub -d -p 9022:22 -p 9080:80 -p 9222:22122 -p 9808:8080 -p 9230:23000 -p 9888:8888 ub:1.0
# iptables -t nat -A POSTROUTING -s STORAGE_SERVER_IP -d TRACKER_SERVER_IP -p tcp -m tcp --dport 22122 -j SNAT --to-source HOST_IP

# docker run -d \
#   -p 9022:22 \
#   -p 8000:8000 \
#   -p 6000:6000 \
#   -v /Users/liuwang/CosyVoice:/root/CosyVoice \
#   liuwang68/ubtool:1.0

# Mount instructions:
# Use the following command to run the container and mount host directory to working directory:
# docker run -it --privileged --name audio-service -d \
#   -p 9022:22 \
#   -p 8080:8080 \
#   -v /path/to/local/directory:/workspace \
#   audio-service:1.0