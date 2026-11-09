FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vimox-router-agent ./cmd/vimox-router-agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vimox-router-agent /usr/local/bin/vimox-router-agent
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/vimox-router-agent", "serve"]
