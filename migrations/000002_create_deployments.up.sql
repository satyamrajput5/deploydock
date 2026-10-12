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
