FROM alpine:latest

ENTRYPOINT ["/bin/shopping-list-manager"]
CMD ["run"]

ADD rel/shopping-list-manager_linux-amd64 /bin/shopping-list-manager
