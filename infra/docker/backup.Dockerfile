# Independent data-server worker; no application request depends on this image.
FROM rclone/rclone:1.75.1 AS rclone
FROM postgres:16-alpine
RUN apk add --no-cache bash ca-certificates util-linux
COPY --from=rclone /usr/local/bin/rclone /usr/local/bin/rclone
COPY infra/scripts/backup-common.sh infra/scripts/backup-data.sh /opt/backup/
RUN chmod 755 /opt/backup/*.sh
ENTRYPOINT ["/bin/bash", "/opt/backup/backup-data.sh"]
