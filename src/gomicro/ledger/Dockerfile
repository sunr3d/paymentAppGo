# Используем образ golang для компиляции бинарника
FROM golang:latest AS build

WORKDIR /app

COPY go.mod ./

RUN go mod tidy

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ledger ./cmd/ledger

# Новый легковестный образ в который забираем только бинарник приложения и запускаем его
FROM alpine:latest

WORKDIR /app

COPY --from=build /app/ledger .

ENTRYPOINT ["./ledger"]