FROM node:23.3.0-alpine3.20 as page
COPY ./web /web
WORKDIR /web
RUN npm install && npm run build

FROM golang:1.22.5-alpine3.20 as builder
RUN apk update
RUN apk add --no-cache git make
COPY . /wsterm
WORKDIR /wsterm
COPY --from=page /web/dist /wsterm/web/dist
RUN go mod tidy
RUN make bin

FROM alpine:3.20
COPY --from=builder /wsterm/wsterm-amd64 /app/bin/wsterm
WORKDIR /app
ENV ENABLE_SSL=true
CMD ["/app/bin/wsterm","-bind",":8080","-fork","/bin/sh"]
