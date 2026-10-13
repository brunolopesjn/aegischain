FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o chaincode .

FROM alpine:3.19
COPY --from=build /app/chaincode /chaincode
CMD ["/chaincode"]