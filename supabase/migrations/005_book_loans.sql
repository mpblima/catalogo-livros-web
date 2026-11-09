begin;
create table catalog.book_loans (
 id uuid primary key default gen_random_uuid(),
 catalog_id uuid not null,
 book_id uuid not null,
 borrower_id text references catalog.users(id),
 borrower_name text not null check(length(borrower_name) between 1 and 200),
 borrower_email text not null check(borrower_email=lower(borrower_email)),
 note text not null default '' check(length(note)<=1000),
 status text not null check(status in ('requested','active','returned','sent','rejected')),
 requested_at timestamptz not null default now(),
 decided_at timestamptz,
 returned_at timestamptz,
 foreign key(catalog_id,book_id) references catalog.books(catalog_id,id) on delete cascade
);
create index book_loans_catalog on catalog.book_loans(catalog_id,requested_at desc);
create index book_loans_borrower on catalog.book_loans(borrower_id,requested_at desc);
create unique index one_active_physical_loan on catalog.book_loans(book_id) where status='active';
alter table catalog.book_loans enable row level security;
revoke all on catalog.book_loans from public,anon,authenticated;
commit;
