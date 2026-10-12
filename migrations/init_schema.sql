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
    status TEXT NOT NULL DEFAULT 'pending',
    port INT UNIQUE,
    container_id TEXT,
    image_tag TEXT,
    url TEXT,
    error_message TEXT,
    build_number INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployments_user_id ON public.deployments(user_id);
CREATE INDEX IF NOT EXISTS idx_deployments_repo_url ON public.deployments(repo_url);

CREATE TABLE IF NOT EXISTS public.deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    log_type TEXT NOT NULL,
    stream TEXT,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_created ON public.deployment_logs(deployment_id, created_at);

CREATE TABLE IF NOT EXISTS public.replicas (
    id SERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    container_id TEXT,
    port INT,
    status TEXT NOT NULL DEFAULT 'running'
);
