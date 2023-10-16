FROM golang:alpine AS build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN go vet -v
RUN go test -v

RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/app

FROM gcr.io/distroless/static-debian12
EXPOSE 8080
COPY --from=build /go/bin/app /
CMD ["/app"]
