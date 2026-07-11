//go:build windows && cgo

package pdfnative

/*
#include <setjmp.h>
#include <stdlib.h>
#include <string.h>

// Some MuPDF static archives are built with fortified/VC helper symbols
// unavailable in MinGW runtime. Provide lightweight compatibility shims.

void *__memcpy_chk(void *dest, const void *src, size_t n, size_t destlen) {
	(void)destlen;
	return memcpy(dest, src, n);
}

void *__memmove_chk(void *dest, const void *src, size_t n, size_t destlen) {
	(void)destlen;
	return memmove(dest, src, n);
}

void *__memset_chk(void *s, int c, size_t n, size_t slen) {
	(void)slen;
	return memset(s, c, n);
}

char *__strcat_chk(char *dest, const char *src, size_t destlen) {
	(void)destlen;
	return strcat(dest, src);
}

void __chk_fail(void) {
	abort();
}

#if defined(_WIN32)
extern int _setjmpex(void *env);
int __intrinsic_setjmpex(void *env, ...) {
	return _setjmpex(env);
}
#endif
*/
import "C"
