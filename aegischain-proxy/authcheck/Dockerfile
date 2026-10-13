FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o authcheck .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /app/authcheck /usr/local/bin/authcheck
EXPOSE 9000
ENTRYPOINT ["authcheck"]