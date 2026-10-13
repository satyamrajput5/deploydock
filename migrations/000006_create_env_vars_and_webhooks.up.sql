CREATE TABLE IF NOT EXISTS public.deployment_env_vars (
    id SERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    encrypted_value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (deployment_id, key)
);

CREATE INDEX IF NOT EXISTS idx_deployment_env_vars_deployment ON public.deployment_env_vars(deployment_id);

CREATE TABLE IF NOT EXISTS public.webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    delivery_id TEXT NOT NULL UNIQUE,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    commit_sha TEXT,
    processing_status TEXT NOT NULL DEFAULT 'received',
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_deployment ON public.webhook_deliveries(deployment_id);
