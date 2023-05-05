FROM golang:latest

COPY . /app

WORKDIR /app

RUN go build -o main .

CMD ["./main"]
