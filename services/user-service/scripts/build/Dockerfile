FROM alpine:latest
MAINTAINER zhufuyi "g.zhufuyi@gmail.com"

# add curl, used for http service checking, can be installed without it if deployed in k8s
RUN apk add curl

COPY configs/ /app/configs/
COPY user_service /app/user_service
RUN chmod +x /app/user_service

# http port
EXPOSE 8080


WORKDIR /app

CMD ["./user_service", "-c", "configs/user_service.yml"]
# if you use the Configuration Center, user_service.yml is changed to the Configuration Center configuration.
#CMD ["./user_service", "-c", "configs/user_service.yml", "-enable-cc"]
