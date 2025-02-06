FROM golang:1.21.0 as base

WORKDIR $GOPATH/src/smallest-golang/app/

COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM gcr.io/distroless/static-debian11

COPY --from=base /main .

CMD ["./main"]