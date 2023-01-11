use light_client_common::state_machine::read_proof_check;
use sp_runtime::{testing::H256, traits::BlakeTwo256, StateVersion};
use sp_trie::trie_types::TrieDBMutV0;
use sp_trie::{
    generate_trie_proof, verify_trie_proof, HashDBT, KeySpacedDBMut, LayoutV0, MemoryDB,
    PrefixedMemoryDB, StorageProof, TrieDBMut, TrieMut,
};
use std::ffi::c_void;

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_new(
    trie_out: *mut *mut c_void,
    db_out: *mut *mut c_void,
    root_out: *mut *mut c_void,
) {
    let db = Box::leak(Box::new(MemoryDB::<BlakeTwo256>::default()));
    let root = Box::leak(Box::new(H256::default()));
    let trie = Box::new(<TrieDBMut<LayoutV0<BlakeTwo256>>>::new(db, root));

    let leaded_trie = Box::into_raw(trie) as *mut c_void;
    let leaded_db = db as *mut _ as *mut c_void;
    let leaded_root = root as *mut _ as *mut c_void;

    *trie_out = leaded_trie;
    *db_out = leaded_db;
    *root_out = leaded_root;
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_free(
    trie_addr: *const c_void,
    db_addr: *const c_void,
    root_addr: *const c_void,
) {
    let _db = Box::from_raw(db_addr as *mut MemoryDB<BlakeTwo256>);
    let _root = Box::from_raw(root_addr as *mut H256);
    let _trie = Box::from_raw(trie_addr as *mut TrieDBMut<LayoutV0<BlakeTwo256>>);
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
    trie_addr: *const c_void,
    key: *const u8,
    key_len: usize,
    value: *const u8,
    value_len: usize,
) {
    let trie = &mut *(trie_addr as *mut TrieDBMut<LayoutV0<BlakeTwo256>>);
    let key = std::slice::from_raw_parts(key, key_len);
    let value = std::slice::from_raw_parts(value, value_len);
    trie.insert(key, value).expect("insert failed");
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_root(
    trie_addr: *const c_void,
    root_out: *mut c_void,
) {
    let trie = &mut *(trie_addr as *mut TrieDBMut<LayoutV0<BlakeTwo256>>);
    let root = trie.root();
    *(root_out as *mut H256) = root.clone();
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_generate_trie_proof(
    db_addr: *const c_void,
    root_addr: *const c_void,
    key: *const u8,
    key_len: usize,
    proof_items_out: *mut *const *mut u8,
    proof_lens_out: *mut *const usize,
    proof_len_out: *mut usize,
) {
    let db = &*(db_addr as *const MemoryDB<BlakeTwo256>);
    let root = &*(root_addr as *const H256);
    let key = std::slice::from_raw_parts(key, key_len).to_owned();
    let proof = generate_trie_proof::<LayoutV0<BlakeTwo256>, _, _, _>(db, *root, vec![&key])
        .expect("generate_trie_proof failed");

    proof_to_raw(proof_items_out, proof_lens_out, proof_len_out, proof);
}

unsafe fn proof_to_raw(
    proof_items_out: *mut *const *mut u8,
    proof_lens_out: *mut *const usize,
    proof_len_out: *mut usize,
    proof: Vec<Vec<u8>>,
) {
    let (mut xs, mut ls): (Vec<_>, Vec<_>) = proof
        .into_iter()
        .map(|mut x| {
            let i = x.len();
            x.shrink_to_fit();
            (x.leak().as_mut_ptr(), i)
        })
        .unzip();

    proof_len_out.write(xs.len());
    xs.shrink_to_fit();
    ls.shrink_to_fit();
    let xs_leaked = xs.leak();
    let ls_leaked = ls.leak();
    proof_items_out.write(xs_leaked.as_mut_ptr());
    proof_lens_out.write(ls_leaked.as_mut_ptr());
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_verify_trie_proof(
    root_addr: *const c_void,
    key: *const u8,
    key_len: usize,
    value: *const u8,
    value_len: usize,
    proof_item: *const *const u8,
    proof_len: *const usize,
    proofs_len: usize,
) -> bool {
    let root = &*(root_addr as *const H256);
    let key = std::slice::from_raw_parts(key, key_len).to_owned();
    let value = std::slice::from_raw_parts(value, value_len).to_owned();
    let proof = proof_from_raw_cloned(proof_item, proof_len, proofs_len);
    verify_trie_proof::<LayoutV0<BlakeTwo256>, _, _, _>(root, &proof, vec![&(key, Some(&value))])
        .is_ok()
}

unsafe fn proof_from_raw_cloned(
    proof_item: *const *const u8,
    proof_len: *const usize,
    proofs_len: usize,
) -> Vec<Vec<u8>> {
    (0..proofs_len)
        .map(|i| {
            let item = std::slice::from_raw_parts(*proof_item.add(i), *proof_len.add(i));
            item.to_owned() // clone data
        })
        .collect::<Vec<_>>()
}

#[no_mangle]
pub unsafe extern "C" fn ext_TrieDBMut_LayoutV0_BlakeTwo256_read_proof_check(
    root_addr: *const c_void,
    key: *const u8,
    key_len: usize,
    proof_item: *const *const u8,
    proof_len: *const usize,
    proofs_len: usize,
    value_out: *mut *mut u8,
    value_len_out: *mut usize,
) -> bool {
    let root = &*(root_addr as *const H256);
    let key = std::slice::from_raw_parts(key, key_len).to_owned();
    let proof = proof_from_raw_cloned(proof_item, proof_len, proofs_len);
    let storage_proof = StorageProof::new(proof);
    let maybe_checked = read_proof_check::<BlakeTwo256, _>(root, storage_proof, vec![&key])
        .map(|mut xs| xs.remove(&key).flatten());
    if let Ok(Some(checked)) = maybe_checked {
        let i = checked.len();
        let checked = checked.leak();
        value_out.write(checked.as_mut_ptr());
        value_len_out.write(i);
        true
    } else {
        false
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use light_client_common::state_machine::read_proof_check;
    use sp_core::storage::ChildInfo;
    use sp_core::Encode;
    use sp_runtime::StateVersion;
    use sp_state_machine::{prove_read, Backend, TrieBackend};
    use sp_trie::trie_types::TrieDBMutV0;
    use sp_trie::{
        generate_trie_proof, verify_trie_proof, HashDBT, KeySpacedDBMut, LayoutV0, MemoryDB,
        PrefixedMemoryDB, StorageProof, TrieDBMut, TrieMut,
    };
    use std::ffi::c_void;

    // run `cargo miri test -p go-export` to check for safety
    #[test]
    fn test_insert() {
        use hex_literal::hex;

        unsafe {
            let mut db_addr: *mut c_void = std::ptr::null_mut();
            let mut root_addr: *mut c_void = std::ptr::null_mut();
            let mut trie_addr: *mut c_void = std::ptr::null_mut();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_new(&mut trie_addr, &mut db_addr, &mut root_addr);

            let mut root = H256::default();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_root(trie_addr, &mut root as *mut _ as *mut c_void);
            assert_eq!(
                root,
                hex!("03170a2e7597b7b7e3d84c05391d139a62b157e78786d8c082f29dcf4c111314").into()
            );

            ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
                trie_addr,
                b"key".as_ptr(),
                3,
                b"value".as_ptr(),
                5,
            );
            let mut root = H256::default();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_root(trie_addr, &mut root as *mut _ as *mut c_void);
            assert_eq!(
                root,
                hex!("434590ba666a2d9ed9f2ca8bde0a2e876b1a744878e8522e9bc2b88c91e6c2c0").into()
            );

            ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
                trie_addr,
                b"key2".as_ptr(),
                4,
                b"value2".as_ptr(),
                6,
            );
            let mut root = H256::default();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_root(trie_addr, &mut root as *mut _ as *mut c_void);
            assert_eq!(
                root,
                hex!("7ff11e72bc15dbb00caf226cf4b2d0f9337dc5a93686b735f475b491873cd2a1").into()
            );

            ext_TrieDBMut_LayoutV0_BlakeTwo256_free(trie_addr, db_addr, root_addr);
        }
    }

    #[test]
    fn test_proof() {
        unsafe {
            let mut db_addr: *mut c_void = std::ptr::null_mut();
            let mut root_addr: *mut c_void = std::ptr::null_mut();
            let mut trie_addr: *mut c_void = std::ptr::null_mut();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_new(&mut trie_addr, &mut db_addr, &mut root_addr);

            ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
                trie_addr,
                b"key".as_ptr(),
                3,
                b"value".as_ptr(),
                5,
            );

            ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
                trie_addr,
                b"key2".as_ptr(),
                4,
                b"value2".as_ptr(),
                6,
            );
            let mut root = H256::default();
            ext_TrieDBMut_LayoutV0_BlakeTwo256_root(trie_addr, &mut root as *mut _ as *mut c_void);

            // construct proof
            let mut proof_items_addr: *const *mut u8 = std::ptr::null_mut();
            let mut proof_lens_addr: *const usize = std::ptr::null_mut();
            let mut proof_len: usize = 0;
            let key = b"key";
            let value = b"value";
            ext_TrieDBMut_LayoutV0_BlakeTwo256_generate_trie_proof(
                db_addr,
                &root as *const _ as *const c_void,
                key.as_ptr() as *const u8,
                key.len(),
                &mut proof_items_addr,
                &mut proof_lens_addr,
                &mut proof_len,
            );

            let proof = proof_from_raw_owned(&proof_items_addr, &proof_lens_addr, proof_len);

            // verify
            verify_trie_proof::<LayoutV0<BlakeTwo256>, _, _, _>(
                &root,
                &proof,
                vec![&(key.to_vec(), Some(value))],
            )
            .expect("verify_trie_proof failed");
            let res = ext_TrieDBMut_LayoutV0_BlakeTwo256_verify_trie_proof(
                &root as *const _ as *const c_void,
                key.as_ptr() as *const u8,
                key.len(),
                value.as_ptr() as *const u8,
                value.len(),
                proof_items_addr as *const *const u8,
                proof_lens_addr,
                proof_len,
            );
            assert!(res, "proof verification failed");

            free_proof(proof_items_addr, proof_lens_addr, proof_len);

            ext_TrieDBMut_LayoutV0_BlakeTwo256_free(trie_addr, db_addr, root_addr);
        }
    }

    unsafe fn proof_from_raw_owned(
        proof_items_addr: &*const *mut u8,
        proof_lens_addr: &*const usize,
        proof_len: usize,
    ) -> Vec<Vec<u8>> {
        (0..proof_len)
            .map(|i| {
                let length = *proof_lens_addr.add(i);
                Vec::from_raw_parts((*proof_items_addr.add(i)) as *mut u8, length, length)
            })
            .collect::<Vec<_>>()
    }

    unsafe fn free_proof(
        proof_items_addr: *const *mut u8,
        proof_lens_addr: *const usize,
        proof_len: usize,
    ) {
        drop(Vec::from_raw_parts(
            proof_lens_addr as *mut usize,
            proof_len,
            proof_len,
        ));
        drop(Vec::from_raw_parts(
            proof_items_addr as *mut *mut u8,
            proof_len,
            proof_len,
        ));
    }

    unsafe fn free_vec(value_out: *mut u8, value_len_out: usize) {
        drop(Vec::from_raw_parts(value_out, value_len_out, value_len_out));
    }

    pub(crate) fn test_db() -> (PrefixedMemoryDB<BlakeTwo256>, H256) {
        let child_info = ChildInfo::new_default(b"sub1");
        let mut root = H256::default();
        let mut mdb = PrefixedMemoryDB::<BlakeTwo256>::default();
        {
            let mut sub_root = Vec::new();
            root.encode_to(&mut sub_root);
            let mut trie = TrieDBMutV0::new(&mut mdb, &mut root);
            trie.insert(child_info.prefixed_storage_key().as_slice(), &sub_root[..])
                .expect("insert failed");
        }
        (mdb, root)
    }

    #[test]
    fn test_read_proof_check() {
        let (mut mdb, mut root) = test_db();
        {
            let mut trie = TrieDBMutV0::from_existing(&mut mdb, &mut root).unwrap();
            trie.insert(b"key", &vec![1u8; 1_000].encode()) // big inner hash
                .expect("insert failed");
            trie.insert(b"key2", &vec![3u8; 16].encode()) // no inner hash
                .expect("insert failed");
            trie.insert(b"key3", &vec![5u8; 100].encode()) // inner hash
                .expect("insert failed");
        }

        let key = b"key3";
        let value = [5u8; 100].as_slice();

        // construct a proof
        let remote_backend = TrieBackend::new(mdb, root);
        let remote_root = remote_backend
            .storage_root(std::iter::empty(), StateVersion::V0)
            .0;
        let remote_proof = prove_read(remote_backend, &[&key]).unwrap();

        // verify
        let val =
            read_proof_check::<BlakeTwo256, _>(&remote_root, remote_proof.clone(), vec![&key])
                .map(|mut xs| xs.remove(key.as_ref()).flatten())
                .ok()
                .flatten()
                .expect("read_proof_check failed");
        assert_eq!(val, value);
        let proof_vec = remote_proof.into_nodes().into_iter().collect::<Vec<_>>();
        println!(
            "remote_proof = [{}]\nroot = {remote_root:?}",
            &proof_vec
                .iter()
                .map(|x| { format!(r#""{}""#, hex::encode(x)) })
                .collect::<Vec<_>>()
                .join(", ")
        );

        let mut proof_items_addr: *const *mut u8 = std::ptr::null_mut();
        let mut proof_lens_addr: *const usize = std::ptr::null_mut();
        let mut proof_len: usize = 0;

        let mut value_out: *mut u8 = std::ptr::null_mut();
        let mut value_len_out: usize = 0;

        unsafe {
            proof_to_raw(
                &mut proof_items_addr,
                &mut proof_lens_addr,
                &mut proof_len,
                proof_vec,
            );

            assert!(
                ext_TrieDBMut_LayoutV0_BlakeTwo256_read_proof_check(
                    &remote_root as *const _ as *const c_void,
                    key.as_ptr() as *const u8,
                    key.len(),
                    proof_items_addr as *const *const u8,
                    proof_lens_addr,
                    proof_len,
                    &mut value_out,
                    &mut value_len_out,
                ),
                "proof verification failed"
            );

            drop(proof_from_raw_owned(
                &proof_items_addr,
                &proof_lens_addr,
                proof_len,
            ));
            free_proof(proof_items_addr, proof_lens_addr, proof_len);
            free_vec(value_out, value_len_out);
        }
    }

    #[test]
    #[ignore]
    fn fuzz_test_trie() {
        use hex_literal::hex;
        use rand::Rng;
        use std::collections::HashMap;

        let mut rng = rand::thread_rng();
        let mut db_addr: *mut c_void = std::ptr::null_mut();
        let mut root_addr: *mut c_void = std::ptr::null_mut();
        let mut trie_addr: *mut c_void = std::ptr::null_mut();
        unsafe {
            ext_TrieDBMut_LayoutV0_BlakeTwo256_new(&mut trie_addr, &mut db_addr, &mut root_addr);
        }

        let mut map = HashMap::new();
        for _ in 0..1000 {
            let key = rng
                .gen::<[u8; 32]>()
                .iter()
                .map(|x| x % 10)
                .collect::<Vec<_>>();
            let value = rng
                .gen::<[u8; 32]>()
                .iter()
                .map(|x| x % 10)
                .collect::<Vec<_>>();
            map.insert(key, value);
        }

        for (key, value) in map.iter() {
            unsafe {
                ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
                    trie_addr,
                    key.as_ptr(),
                    key.len(),
                    value.as_ptr(),
                    value.len(),
                );
            }
        }
    }
}
