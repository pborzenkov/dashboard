FROM golang:1.15 as build

WORKDIR /go/src
ADD . /go/src

RUN CGO_ENABLED=0 go build \
	-mod readonly \
	-o /go/bin/dashboard

FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.source https://github.com/pborzenkov/dashboard

COPY --from=build /go/bin/dashboard /

USER nonroot

CMD ["/dashboard"]
