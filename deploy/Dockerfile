FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /symphony ./cmd/symphony

FROM alpine:3.21
RUN apk add --no-cache git openssh-client ca-certificates
COPY --from=build /symphony /usr/local/bin/symphony
ENTRYPOINT ["symphony"]
