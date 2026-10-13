ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT 'main';
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS auto_deploy BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS webhook_id TEXT;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS webhook_secret_enc TEXT;
ALTER TABLE public.deployments ADD COLUMN IF NOT EXISTS current_build_id INT;

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

ALTER TABLE public.deployment_logs ADD COLUMN IF NOT EXISTS build_id INT REFERENCES public.deployment_builds(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_deployment_logs_build_id ON public.deployment_logs(build_id);
