#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

void ext_TrieDBMut_LayoutV0_BlakeTwo256_new(void **trie_out, void **db_out, void **root_out);

void ext_TrieDBMut_LayoutV0_BlakeTwo256_free(const void *trie_addr,
                                             const void *db_addr,
                                             const void *root_addr);

void ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(const void *trie_addr,
                                               const uint8_t *key,
                                               uintptr_t key_len,
                                               const uint8_t *value,
                                               uintptr_t value_len);

void ext_TrieDBMut_LayoutV0_BlakeTwo256_root(const void *trie_addr, void *root_out);

void ext_TrieDBMut_LayoutV0_BlakeTwo256_generate_trie_proof(const void *db_addr,
                                                            const void *root_addr,
                                                            const uint8_t *key,
                                                            uintptr_t key_len,
                                                            uint8_t *const **proof_items_out,
                                                            const uintptr_t **proof_lens_out,
                                                            uintptr_t *proof_len_out);

bool ext_TrieDBMut_LayoutV0_BlakeTwo256_verify_trie_proof(const void *root_addr,
                                                          const uint8_t *key,
                                                          uintptr_t key_len,
                                                          const uint8_t *value,
                                                          uintptr_t value_len,
                                                          const uint8_t *const *proof_item,
                                                          const uintptr_t *proof_len,
                                                          uintptr_t proofs_len);

bool ext_TrieDBMut_LayoutV0_BlakeTwo256_read_proof_check(const void *root_addr,
                                                         const uint8_t *key,
                                                         uintptr_t key_len,
                                                         const uint8_t *const *proof_item,
                                                         const uintptr_t *proof_len,
                                                         uintptr_t proofs_len,
                                                         uint8_t **value_out,
                                                         uintptr_t *value_len_out);
