FROM alpine:latest

ENTRYPOINT ["/bin/shopping-list-manager"]
CMD ["run"]

WORKDIR /workspace
# for storing the barcode-db.json
VOLUME ["/workspace"]

ADD rel/shopping-list-manager_linux-amd64 /bin/shopping-list-manager
