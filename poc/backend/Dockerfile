FROM golang:1.21.0 as base

WORKDIR $GOPATH/src/smallest-golang/app/

COPY . .

RUN go mod init
# RUN go mod download
# RUN go mod verify

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM gcr.io/distroless/static-debian11

COPY --from=base /main .

CMD ["./main"]