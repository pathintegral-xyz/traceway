-- D1's HTTP API previously bound Go booleans as JSON booleans, which D1
-- stored as TEXT 'true'/'false' even in INTEGER-affinity columns. Normalize
-- the small transactional tables so existing rows match 0/1 predicates.
UPDATE notification_channels SET enabled = CASE lower(enabled) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(enabled) = 'text' AND lower(enabled) IN ('true', 'false');
UPDATE notification_rules SET enabled = CASE lower(enabled) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(enabled) = 'text' AND lower(enabled) IN ('true', 'false');
UPDATE projects SET drop_healthy_healthchecks = CASE lower(drop_healthy_healthchecks) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(drop_healthy_healthchecks) = 'text' AND lower(drop_healthy_healthchecks) IN ('true', 'false');
UPDATE widget_groups SET is_default = CASE lower(is_default) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(is_default) = 'text' AND lower(is_default) IN ('true', 'false');
UPDATE widget_group_widgets SET is_starred = CASE lower(is_starred) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(is_starred) = 'text' AND lower(is_starred) IN ('true', 'false');
UPDATE refresh_tokens SET revoked = CASE lower(revoked) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(revoked) = 'text' AND lower(revoked) IN ('true', 'false');
UPDATE refresh_tokens SET used = CASE lower(used) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(used) = 'text' AND lower(used) IN ('true', 'false');
UPDATE personal_access_tokens SET revoked = CASE lower(revoked) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(revoked) = 'text' AND lower(revoked) IN ('true', 'false');
UPDATE user_contact_methods SET enabled = CASE lower(enabled) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(enabled) = 'text' AND lower(enabled) IN ('true', 'false');
UPDATE user_contact_methods SET verified = CASE lower(verified) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(verified) = 'text' AND lower(verified) IN ('true', 'false');
UPDATE synthetic_checks SET enabled = CASE lower(enabled) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(enabled) = 'text' AND lower(enabled) IN ('true', 'false');
UPDATE status_pages SET is_public = CASE lower(is_public) WHEN 'true' THEN 1 ELSE 0 END WHERE typeof(is_public) = 'text' AND lower(is_public) IN ('true', 'false');
