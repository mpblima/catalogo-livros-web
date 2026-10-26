begin;
create table catalog.email_outbox (
 id uuid primary key default gen_random_uuid(), catalog_id uuid not null references catalog.catalogs(id),
 recipient text not null, subject text not null, html text not null,
 attempts integer not null default 0, next_attempt timestamptz not null default now(),
 delivered_at timestamptz, created_at timestamptz not null default now()
);
create index pending_email on catalog.email_outbox(next_attempt) where delivered_at is null;
create table catalog.audit_events (
 id bigint generated always as identity primary key, actor_id text not null references catalog.users(id),
 action text not null, resource_id text not null, created_at timestamptz not null default now()
);
create table catalog.privacy_requests (
 id uuid primary key default gen_random_uuid(), user_id text not null references catalog.users(id),
 kind text not null check(kind='erasure'), status text not null default 'pending',
 created_at timestamptz not null default now(), unique(user_id,kind)
);
alter table catalog.email_outbox enable row level security;
alter table catalog.audit_events enable row level security;
alter table catalog.privacy_requests enable row level security;
revoke all on all tables in schema catalog from public,anon,authenticated;
commit;
