FROM golang:alpine

WORKDIR /app

COPY . /app

RUN go build -o allora_offchain_node

CMD ["go", "run", "allora_offchain_node"]
