FROM golang:1.16-buster as builder
ENV APP_USER app
ENV APP_HOME /go/src/sns2ps-api
RUN groupadd $APP_USER && useradd -m -g $APP_USER -l $APP_USER
RUN mkdir -p $APP_HOME && chown -R $APP_USER:$APP_USER $APP_HOME
WORKDIR $APP_HOME
USER $APP_USER
COPY * .
RUN go mod download
RUN go mod verify
RUN go build -o sns2ps-api
FROM debian:buster
FROM golang:1.16-buster
ENV APP_USER app
ENV BUILD_APP_HOME /go/src/sns2ps-api
ENV APP_HOME /app
ENV PORT 8080
RUN groupadd $APP_USER && useradd -m -g $APP_USER -l $APP_USER
RUN mkdir -p $APP_HOME
WORKDIR /
COPY sample_content/ sample_content/
WORKDIR $APP_HOME
COPY --chown=0:0 --from=builder $BUILD_APP_HOME/sns2ps-api $APP_HOME
EXPOSE $PORT
USER $APP_USER
CMD ["./sns2ps-api"]