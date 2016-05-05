package main

/*
extern char *go_get_callback(int doc_index, int *doc_len, void *user_data);
extern void go_release_callback(char *buf, void *user_data);

const char *get_callback(int doc_index, int *doc_len, void *user_data)
{
	return go_get_callback(doc_index, doc_len, user_data);
}

void release_callback(const char *buf, void *user_data)
{
	go_release_callback((char *)buf, user_data);
}
*/
import "C"
