DROP TABLE IF EXISTS link_uploaded_files;

ALTER TABLE links
    DROP COLUMN IF EXISTS target_folder_path,
    DROP COLUMN IF EXISTS link_type;
