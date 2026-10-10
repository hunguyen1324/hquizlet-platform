# PostgreSQL client tools only; this container never starts a database server.
FROM alpine:3.22
RUN apk add --no-cache postgresql16-client python3 ca-certificates
COPY infra/scripts/neon-backup.py /opt/backup/neon-backup.py
ENTRYPOINT ["python3", "-u", "/opt/backup/neon-backup.py"]
