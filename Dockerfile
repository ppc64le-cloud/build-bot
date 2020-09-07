FROM golang:1.15 as builder

WORKDIR /go/src/github.com/ppc64le-cloud/build-bot

COPY . .

RUN CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w' .

FROM gcr.io/distroless/static

COPY --from=builder /go/src/github.com/ppc64le-cloud/build-bot/build-bot /build-bot

ENTRYPOINT ["/build-bot"]