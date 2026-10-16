CREATE TABLE llm_usage (
    day date NOT NULL,
    provider text NOT NULL,
    stage text NOT NULL,
    model text NOT NULL,
    calls integer NOT NULL DEFAULT 0,
    input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    cost_usd numeric(14, 8) NOT NULL DEFAULT 0,
    PRIMARY KEY (day, provider, stage, model)
);
