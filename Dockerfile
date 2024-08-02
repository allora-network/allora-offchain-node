FROM golang:alpine

WORKDIR /

COPY . /node

RUN go mod download

RUN go build -o /node/allora_offchain_node

CMD ["/node/allora_offchain_node"]
