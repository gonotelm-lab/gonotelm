# Domain Error Code Allocation

## Rule

1. Domain error code should be allocated in the domain package.
2. Domain error code is a 6-number digits, format is like xxx-xxx.
3. The first 3 numbers is the domain code, the last 3 numbers are the sequential error code, starts from 001.

## Domain Code Allocation

| Domain | Code |
|--------|------|
| notebook | 100 |
| chat | 101 |
| artifact | 102 |
| source | 103 |
| worker | 104 |
| sandbox | 105 |
