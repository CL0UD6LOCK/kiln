CREATE TABLE IF NOT EXISTS orders(
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_name   VARCHAR(64)     NOT NULL,
    detail      JSON            NULL,
    state       VARCHAR(16)     NOT NULL DEFAULT 'new',
    retry_count INT             NOT NULL DEFAULT 0,
    note        TEXT NULL,
    paid_at     DATETIME(3)     NULL,
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    updated_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE
CURRENT_TIMESTAMP(3),
    PRIMARY KEY(id),
    KEY idx_state_paid(state,paid_at)
    )ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;