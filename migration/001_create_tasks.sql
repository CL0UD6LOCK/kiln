CREATE TABLE IF NOT EXISTS tasks (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    type         VARCHAR(64)     NOT NULL,
    payload      JSON            NULL,
    status       VARCHAR(64)     NOT NULL DEFAULT 'new',
    attempts     INT             NULL,
    max_attempts INT             NULL,
    run_at       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    last_error   TEXT            NULL,
    created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY(id),
    KEY idx_payload_status (status,payload)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;