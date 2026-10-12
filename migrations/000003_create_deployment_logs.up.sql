CREATE TABLE IF NOT EXISTS public.deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    log_type TEXT NOT NULL,
    stream TEXT,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_created ON public.deployment_logs(deployment_id, created_at);
