ALTER TABLE `book_title_suffix` ADD INDEX `IX_title_suffix_book_id` (`title_suffix`, `book_id`);
ALTER TABLE `book_author_suffix` ADD INDEX `IX_author_suffix_book_id` (`author_suffix`, `book_id`);
