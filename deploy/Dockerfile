FROM node:22-alpine AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /frontend/dist ./internal/status/dist/
RUN CGO_ENABLED=0 go build -o /symphony ./cmd/symphony

FROM alpine:3.21
RUN apk add --no-cache git openssh-client ca-certificates
COPY --from=build /symphony /usr/local/bin/symphony
ENTRYPOINT ["symphony"]
