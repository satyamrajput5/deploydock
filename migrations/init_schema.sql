CREATE TABLE IF NOT EXISTS public.users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.deployments (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES public.users(id) ON DELETE SET NULL,
    repo_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    auto_deploy BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_id TEXT,
    webhook_secret_enc TEXT,
    current_build_id INT,
    url TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    port INT UNIQUE,
    container_id TEXT,
    image_tag TEXT,
    error_message TEXT,
    build_number INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT 'main';
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS auto_deploy BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS webhook_id TEXT;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS webhook_secret_enc TEXT;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS current_build_id INT;

CREATE INDEX IF NOT EXISTS idx_deployments_user_id ON public.deployments(user_id);
CREATE INDEX IF NOT EXISTS idx_deployments_repo_url ON public.deployments(repo_url);

CREATE TABLE IF NOT EXISTS public.deployment_builds (
    id SERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    commit_sha TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    image_tag TEXT,
    container_id TEXT,
    port INT UNIQUE,
    error_message TEXT,
    trigger_type TEXT NOT NULL DEFAULT 'manual',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployment_builds_deployment_id ON public.deployment_builds(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_builds_status ON public.deployment_builds(status);

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

CREATE TABLE IF NOT EXISTS public.deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    build_id INT REFERENCES public.deployment_builds(id) ON DELETE CASCADE,
    log_type TEXT NOT NULL,
    stream TEXT,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE public.deployment_logs ADD COLUMN IF NOT EXISTS build_id INT REFERENCES public.deployment_builds(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_created ON public.deployment_logs(deployment_id, created_at);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_build_id ON public.deployment_logs(build_id);

CREATE TABLE IF NOT EXISTS public.replicas (
    id SERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    container_id TEXT,
    port INT,
    status TEXT NOT NULL DEFAULT 'running'
);
