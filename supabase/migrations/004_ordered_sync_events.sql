begin;
-- Serialize sequence allocation and commits per catalog, including mixed-version writers.
create or replace function catalog.record_book_event()returns trigger language plpgsql as $$begin perform pg_advisory_xact_lock(hashtextextended(NEW.catalog_id::text,0));insert into catalog.sync_events(catalog_id,book_id,kind)values(NEW.catalog_id,NEW.id,'book');return NEW;end$$;
create or replace function catalog.record_comment_event()returns trigger language plpgsql as $$begin perform pg_advisory_xact_lock(hashtextextended(NEW.catalog_id::text,0));insert into catalog.sync_events(catalog_id,book_id,comment_id,kind)values(NEW.catalog_id,NEW.book_id,NEW.id,'comment');return NEW;end$$;
commit;
