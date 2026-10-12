CREATE TABLE IF NOT EXISTS public.replicas (
    id SERIAL PRIMARY KEY,
    deployment_id INT NOT NULL REFERENCES public.deployments(id) ON DELETE CASCADE,
    container_id TEXT,
    port INT,
    status TEXT NOT NULL DEFAULT 'running'
);
