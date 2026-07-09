DROP TABLE IF EXISTS link_index_files;

ALTER TABLE links
    DROP COLUMN IF EXISTS index_file_enabled;
