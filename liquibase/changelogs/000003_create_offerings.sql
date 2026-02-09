--liquibase formatted sql

--changeset mancalvo:000003-create-offerings
--comment: Offerings catalog
CREATE TABLE offerings (
    id CHAR(20) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    duration_days INT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

--rollback DROP TABLE offerings;
