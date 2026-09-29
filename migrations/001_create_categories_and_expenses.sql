CREATE TABLE categories (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    CONSTRAINT categories_name_not_empty CHECK (btrim(name, E' \t\n\r\f' || chr(11)) <> '')
);

CREATE TABLE expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id BIGINT NOT NULL,
    amount_kopecks BIGINT NOT NULL,
    expense_date DATE NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    CONSTRAINT expenses_category_fk FOREIGN KEY (category_id) REFERENCES categories (id) ON DELETE RESTRICT,
    CONSTRAINT expenses_amount_positive CHECK (amount_kopecks > 0),
    CONSTRAINT expenses_date_finite CHECK (isfinite(expense_date))
);

CREATE INDEX expenses_date_id_idx ON expenses (expense_date DESC, id DESC);
CREATE INDEX expenses_category_date_id_idx ON expenses (category_id, expense_date DESC, id DESC);
