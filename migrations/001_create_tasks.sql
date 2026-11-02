CREATE TABLE IF NOT EXISTS tasks (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    type         VARCHAR(64)     NOT NULL,
    payload      JSON            NOT NULL,
    status       VARCHAR(16)     NOT NULL DEFAULT 'pending',
    attempts     INT             NOT NULL DEFAULT 0,
    max_attempts INT             NOT NULL DEFAULT 3,
    run_at       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    last_error   TEXT            NULL,
    created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY(id),
    KEY idx_status_run_at (status,run_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;