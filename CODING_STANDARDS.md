# Coding Standards for polymarket-go

This document outlines the coding standards and requirements for this project. **Please read carefully and follow strictly.**

## 1. Language Requirements

### 1.1 Code Language
- **All code, comments, and documentation must be in English.**
- No Chinese characters allowed in:
  - Variable names
  - Function names
  - Comments
  - Error messages
  - Log messages
  - Documentation

**Bad:**
```go
// 获取用户余额
func GetBalance() {
    log.Println("查询成功")
}
```

**Good:**
```go
// Get user balance
func GetBalance() {
    log.Println("Query successful")
}
```

## 2. Error Handling

### 2.1 Never Ignore Errors
- **Always handle errors properly**
- Never use `_ = someFunction()` to ignore errors unless there's a very good reason
- If you must ignore an error, add a comment explaining why

**Bad:**
```go
_ = client.StopAutoRedeem()
```

**Good:**
```go
if err := client.StopAutoRedeem(); err != nil {
    return fmt.Errorf("failed to stop auto redeem: %w", err)
}
```

### 2.2 Error Wrapping
- Always wrap errors with context using `%w`
- Use descriptive error messages

**Example:**
```go
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

## 3. Code Quality

### 3.1 Simplicity First
- **Keep examples simple and straightforward**
- Don't overcomplicate with too many variations
- One clear, complete example is better than multiple confusing ones

**Bad:**
```go
// Example 1: Only auto-redeem
// Example 2: Only auto-merge
// Example 3: Both
// Example 4: Manual control
// Example 5: Production pattern
```

**Good:**
```go
// One complete example showing both features together
```

### 3.2 No Redundant Code
- Don't create multiple clients for demonstration purposes
- Don't repeat the same concept in different ways
- Examples should be production-ready, not just demonstrations

### 3.3 API Design
- **If config enables a feature, it should auto-start**
  - Don't require users to configure AND manually start
  - "脱裤子放屁" (redundant) design is unacceptable

**Bad:**
```go
// User has to configure AND call Start?
client := NewClient(WithAutoRedeem(config))
client.StartAutoRedeem() // Why? It's already configured!
```

**Good:**
```go
// Configuration = Activation
client := NewClient(WithAutoRedeem(config))
// Service automatically started
defer client.Close() // Clean shutdown
```

### 3.4 Resource Management
- Provide `Close()` method for cleanup
- Use `defer` in examples to demonstrate proper cleanup
- Follow Go standard library patterns (like `http.Server.Close()`)

## 4. Development Practices

### 4.1 Clean Build Artifacts
- **Delete compiled binaries after building**
- Don't leave `main`, `04_auto_management`, etc. in the repository
- Add to `.gitignore`:
  ```
  # Compiled binaries
  /examples/*/main
  /examples/01_balance_query/01_balance_query
  /examples/02_complementary_token_query/02_complementary_token_query
  /examples/03_order_conversion/03_order_conversion
  /examples/04_auto_management/04_auto_management
  ```

### 4.2 Testing
- Always compile after making changes: `go build ./...`
- Clean up afterwards: `find . -name "main" -type f -delete`

## 5. Documentation

### 5.1 TODO Comments
- When code has known limitations, add TODO comments
- Be specific about what needs to be fixed

**Example:**
```go
// TODO: Implement pagination to fetch all positions if user has more than 500 positions
// Currently limited to 500 positions per API constraint
positions, err := c.dataClient.GetPositions(ctx, &polymarketdata.GetPositionsParams{
    User:  c.funderAddr.Hex(),
    Limit: 500, // API max limit
})
```

### 5.2 Function Comments
- All exported functions must have comments
- Comments should explain WHAT and WHY, not just HOW
- Use proper godoc format

**Example:**
```go
// Close stops all background services and releases resources
// This should be called when the client is no longer needed to ensure graceful shutdown
func (c *Client) Close() error {
    return c.StopAutoManagement()
}
```

## 6. Production-Ready Code

### 6.1 This is Production Code
- **Treat every line as production code**
- No "quick and dirty" solutions
- No placeholder implementations
- No "we'll fix it later" attitude

### 6.2 Serious Mindset
- Take criticism seriously and improve
- Don't make the same mistake twice
- Quality over speed

## 7. Git Practices

### 7.1 Clean Repository
- Don't commit:
  - Compiled binaries
  - Temporary files
  - IDE-specific files (unless shared by team)
  - Test output files

### 7.2 Commit Messages
- Use clear, descriptive commit messages
- Follow conventional commit format when appropriate

## 8. Common Mistakes to Avoid

### ❌ Don't Do This:
1. Using Chinese in code
2. Ignoring errors with `_`
3. Creating redundant APIs (config + start method)
4. Leaving compiled binaries in repository
5. Overcomplicating examples
6. "Quick and dirty" implementations

### ✅ Do This Instead:
1. Everything in English
2. Handle all errors properly
3. Configuration = Activation pattern
4. Clean build artifacts
5. Simple, clear examples
6. Production-quality code

## Summary

**Key Principles:**
1. **English only** - No Chinese anywhere
2. **Handle errors** - Never ignore them
3. **Keep it simple** - Don't overcomplicate
4. **Clean up** - Delete build artifacts
5. **Production mindset** - This is serious code
6. **No redundancy** - Avoid "脱裤子放屁" designs

**When in doubt:**
- Is it simple enough?
- Is it production-ready?
- Would I use this code in a real project?

If the answer to any of these is "no", improve it.
