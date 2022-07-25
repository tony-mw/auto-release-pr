FROM golang:1.16-alpine

WORKDIR /app

COPY . .

RUN go mod download

#RUN go build -o ./go-git-action
RUN go install bitbucket.dentsplysirona.com/atopoc/auto-release-pr

ENTRYPOINT [ "/bin/sh" ]