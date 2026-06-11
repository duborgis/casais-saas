FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /harmonia .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates && adduser -D -u 1000 harmonia
USER harmonia

COPY --from=build /harmonia /usr/local/bin/harmonia

EXPOSE 8080
CMD ["harmonia"]
