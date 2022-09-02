FROM golang:1.19-alpine

RUN apk add --no-cache git

LABEL "com.github.actions.name"="notion2md"
LABEL "com.github.actions.description"="Notion blog articles database to hugo-style markdown."
LABEL "repository"="https://github.com/rxrw/notion2md"
LABEL "maintainer"="Jens <rxrw@me.com>"

WORKDIR /usr/src/app

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Build the Go app
RUN go build -o ./bin/notion2md main.go

ENTRYPOINT ["/usr/src/app/bin/notion2md"]
