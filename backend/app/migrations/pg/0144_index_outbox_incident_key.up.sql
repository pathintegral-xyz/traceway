CREATE UNIQUE INDEX notification_outbox_incident_key_idx ON notification_outbox(incident_key) WHERE incident_key <> '';
