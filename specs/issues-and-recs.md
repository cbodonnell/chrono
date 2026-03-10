# Issues & Recommendations

## Progress tracker

Completed:
- 

## Issues
                                                                                                                    
1. Potential Bug: Badger Scan callback uses unstable key reference (Medium)                                           
                                                                                                                    
In badger_index.go:48 and badger_index.go:95:                                                                         
if !fn(it.Item().Key()) {  // Key() returns bytes that may be reused!                                                 
                                                                                                                    
BadgerDB's Key() returns a slice that can be reused after Next(). If the callback stores the key (which Reindex does  
at entity_store.go:347), it may be corrupted. The KV store correctly uses ValueCopy, but the index store callbacks may
have issues.                                                                                                         
                                                                                                                    
Recommendation: Copy the key before passing to callback, or document that callbacks must copy if they retain          
references.                                                                                                           
                                                                                                                    
2. Reindex silently swallows errors (Medium)                                                                          
                                                                                                                    
In entity_store.go:364-395, errors during reindexing are logged implicitly via return true:                           
if err := s.serializer.Unmarshal(value, &e); err != nil {                                                             
    return true // Skip malformed entities - no logging!                                                              
}                                                                                                                     
                                                                                                                    
Recommendation: At minimum, log these errors. Consider returning an error or collecting failures.                     
                                                                                                                    
3. Range query performance: full prefix scan (Low-Medium)                                                             
                                                                                                                    
For OpLt/OpLte/OpGt/OpGte in query.go:165-185, the code scans the entire field prefix and filters in-memory:          
err := s.indexStore.ScanPrefix(prefix, func(key []byte) bool {                                                        
    // ... compare each key                                                                                           
})                                                                                                                    
                                                                                                                    
For large datasets with selective range queries, this is inefficient. The encoding is already order-preserving, so    
bounded scans could be used.                                                                                          
                                                                                                                    
Recommendation: Build proper range bounds for comparison operators similar to how OpEq does it.                       
                                                                                                                    
4. Missing index on _by_id for range end in Get() (Low)                                                               
                                                                                                                    
In entity_store.go:106:                                                                                               
end := s.keyBuilder.BuildByIDRangeEnd(entityType, entityID, 1<<62)                                                    
                                                                                                                    
1<<62 is arbitrary. Using math.MaxInt64 would be more correct (though unlikely to matter in practice).                
                                                                                                                    
5. KeyBuilder buffer not concurrency-safe (Low)                                                                       
                                                                                                                    
KeyBuilder uses an internal buffer and Reset(). If shared across goroutines (which it currently isn't), it would cause
races. The pattern of creating new builders is fine, but the struct's design invites misuse.                         
                                                                                                                    
Recommendation: Either document "not safe for concurrent use" or make BuildKey methods create local buffers.          
                                                                                                                    
6. HTTP API missing pagination for history (Low)                                                                      
                                                                                                                    
GetHistory returns all versions when limit is not set. For entities with many versions, this could be problematic.    
                                                                                                                    
Recommendation: Consider adding cursor-based pagination or a default limit.                                           
                                                                                                                    
7. TODO in code worth addressing (Note)                                                                               
                                                                                                                    
entity_store.go:34:                                                                                                   
// TODO: figure out if this should happen here or not.                                                                
                                                                                                                    
The auto-reindex on startup is convenient but surprising. Consider making it opt-in via config.                       
                                                                                                                    
query.go:53:                                                                                                          
// TODO: figure out multi-tenancy within the database                                                                 
                                                                                                                    
Worth tracking for future design.                                                                                     
                                                                                                                    
8. Query Reverse and IncludeHistory not exposed in HTTP API (Low)                                                     
                                                                                                                    
The Query struct has Reverse and IncludeHistory fields, but QueryRequest doesn't expose them.                         
                                                                                                                    
Recommendation: Add these to the API for feature parity.