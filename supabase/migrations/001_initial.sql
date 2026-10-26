begin;
create schema if not exists catalog;
-- Private schema: only the Go server connects. No grants to anon/authenticated.
create table catalog.users (
 id text primary key, email text not null unique check(email=lower(email)),
 name text not null, created_at timestamptz not null default now()
);
create table catalog.access_grants (
 email text primary key check(email=lower(email)), enabled boolean not null default true,
 universal boolean not null default false, admin boolean not null default false, granted_by text not null references catalog.users(id),
 updated_at timestamptz not null default now()
);
create table catalog.service_settings (singleton boolean primary key default true check(singleton), initialized boolean not null default false);
insert into catalog.service_settings(singleton) values(true);
alter table catalog.service_settings enable row level security;
create table catalog.catalogs (
 id uuid primary key default gen_random_uuid(), owner_id text not null unique references catalog.users(id),
 name text not null, created_at timestamptz not null default now()
);
create table catalog.members (
 catalog_id uuid not null references catalog.catalogs(id) on delete cascade,
 email text not null check(email=lower(email)), created_by text not null references catalog.users(id),
 created_at timestamptz not null default now(), primary key(catalog_id,email)
);
create table catalog.books (
 id uuid primary key default gen_random_uuid(), catalog_id uuid not null references catalog.catalogs(id),
 title text not null check(length(title) between 1 and 500),
 kind text not null check(kind in ('physical','digital')), format text not null default '',
 path text not null default '', filename text not null default '', category text not null default '',
 metadata jsonb not null default '{}' check(jsonb_typeof(metadata)='object'),
 summary text not null default '', cover_url text not null default '',
 favorite boolean not null default false, reading boolean not null default false,
 version bigint not null default 1, updated_by text not null references catalog.users(id),
 updated_at timestamptz not null default now(), deleted_at timestamptz,
 unique(catalog_id,id)
);
create index books_catalog on catalog.books(catalog_id) where deleted_at is null;
create table catalog.comments (
 id uuid primary key default gen_random_uuid(), catalog_id uuid not null,
 book_id uuid not null, author_id text not null references catalog.users(id),
 body text not null check(length(body) between 1 and 20000), created_at timestamptz not null default now(),
 foreign key(catalog_id,book_id) references catalog.books(catalog_id,id)
);
create index comments_book on catalog.comments(catalog_id,book_id,created_at);
-- Reserved for bidirectional desktop mapping; local IDs are scoped per installation.
create table catalog.sync_links (
 catalog_id uuid not null, device_id uuid not null, local_id text not null,
 book_id uuid not null, base_version bigint not null,
 primary key(catalog_id,device_id,local_id),
 foreign key(catalog_id,book_id) references catalog.books(catalog_id,id)
);
alter table catalog.users enable row level security;
alter table catalog.access_grants enable row level security;
alter table catalog.catalogs enable row level security;
alter table catalog.members enable row level security;
alter table catalog.books enable row level security;
alter table catalog.comments enable row level security;
alter table catalog.sync_links enable row level security;
revoke all on schema catalog from public, anon, authenticated;
revoke all on all tables in schema catalog from public, anon, authenticated;
commit;
