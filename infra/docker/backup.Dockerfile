# Independent data-server worker; no application request depends on this image.
FROM rclone/rclone:1.75.1
RUN apk add --no-cache bash ca-certificates util-linux python3
COPY infra/scripts/backup-common.sh infra/scripts/backup-data.sh infra/scripts/backup-quota.py /opt/backup/
RUN chmod 755 /opt/backup/*.sh
ENTRYPOINT ["/bin/bash", "/opt/backup/backup-data.sh"]
