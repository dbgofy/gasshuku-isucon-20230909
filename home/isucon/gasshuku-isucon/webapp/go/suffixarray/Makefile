.PHONY: all
all: init insert dump

.PHONY: init
init:
	cd ../../sql && bash init_db.sh

.PHONY: insert
insert:
	time go run .

.PHONY: dump
dump:
	mysqldump -t --disable-keys -uisucon -pisucon -hlocalhost isulibrary book_title_suffix > ../../sql/2_book_title_suffix.sql
	mysqldump -t --disable-keys -uisucon -pisucon -hlocalhost isulibrary book_author_suffix > ../../sql/2_book_author_suffix.sql
