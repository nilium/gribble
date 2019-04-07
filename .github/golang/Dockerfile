FROM golang:1

LABEL "com.github.actions.name"="Go"
LABEL "com.github.actions.description"="Run the Go tool"
LABEL "com.github.actions.icon"="command"
LABEL "com.github.actions.color"="blue"

LABEL "repository"="http://github.com/nilium/gribble"
LABEL "homepage"="http://github.com/nilium/gribble"
LABEL "maintainer"="Noel Cower <ncower@gmail.com>"

ENV GO_PACKAGE ''

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
