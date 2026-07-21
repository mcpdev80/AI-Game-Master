FROM golang:1.25.8-bookworm AS build

WORKDIR /app

ARG APP_VERSION=dev
ARG GIT_COMMIT=dev
ARG BUILD_DATE=unknown

COPY go.mod ./
COPY go.sum ./
RUN /usr/local/go/bin/go mod download

COPY . .
RUN CGO_ENABLED=0 /usr/local/go/bin/go build \
	-ldflags="-X 'github.com/marcel/dungeon-master/api/internal/httpapi.buildVersion=${APP_VERSION}' -X 'github.com/marcel/dungeon-master/api/internal/httpapi.buildCommit=${GIT_COMMIT}' -X 'github.com/marcel/dungeon-master/api/internal/httpapi.buildDate=${BUILD_DATE}'" \
	-o /bin/dungeon-master-api ./cmd/server

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache poppler-utils mupdf-tools
COPY --from=build /bin/dungeon-master-api /usr/local/bin/dungeon-master-api

EXPOSE 8080

CMD ["dungeon-master-api"]
