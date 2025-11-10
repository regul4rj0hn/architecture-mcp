# Repository Pattern

## Overview

The Repository pattern encapsulates the logic needed to access data sources. It centralizes common data access functionality, providing better maintainability and decoupling the infrastructure or technology used to access databases from the domain model layer.

## Intent

- Encapsulate data access logic
- Provide a more object-oriented view of the persistence layer
- Support unit testing by providing a mockable interface
- Centralize data access policies

## Structure

```
┌─────────────────┐    ┌──────────────────────┐
│   Domain Model  │    │    Repository        │
│                 │◄───┤    Interface         │
└─────────────────┘    └──────────────────────┘
                                 △
                                 │
                       ┌──────────────────────┐
                       │   Concrete           │
                       │   Repository         │
                       │   Implementation     │
                       └──────────────────────┘
                                 │
                       ┌──────────────────────┐
                       │   Data Source        │
                       │   (Database, API,    │
                       │    File System)      │
                       └──────────────────────┘
```

## Implementation

### Repository Interface

```go
type UserRepository interface {
    GetByID(id string) (*User, error)
    GetByEmail(email string) (*User, error)
    Create(user *User) error
    Update(user *User) error
    Delete(id string) error
    List(filters UserFilters) ([]*User, error)
}
```

### Concrete Implementation

```go
type PostgreSQLUserRepository struct {
    db *sql.DB
}

func (r *PostgreSQLUserRepository) GetByID(id string) (*User, error) {
    query := "SELECT id, email, name, created_at FROM users WHERE id = $1"
    row := r.db.QueryRow(query, id)
    
    var user User
    err := row.Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrUserNotFound
        }
        return nil, err
    }
    
    return &user, nil
}

func (r *PostgreSQLUserRepository) Create(user *User) error {
    query := `
        INSERT INTO users (id, email, name, created_at) 
        VALUES ($1, $2, $3, $4)
    `
    _, err := r.db.Exec(query, user.ID, user.Email, user.Name, user.CreatedAt)
    return err
}
```

## Benefits

### Testability
- Easy to mock for unit testing
- Isolates business logic from data access concerns
- Enables testing without database dependencies

### Flexibility
- Can switch between different data sources
- Supports multiple implementations (SQL, NoSQL, in-memory)
- Enables data source-specific optimizations

### Maintainability
- Centralizes data access logic
- Reduces code duplication
- Provides clear separation of concerns

## Best Practices

### Interface Design
- Keep interfaces focused and cohesive
- Use domain-specific method names
- Return domain objects, not data transfer objects

### Error Handling
- Define domain-specific error types
- Handle data source errors appropriately
- Provide meaningful error messages

### Performance Considerations
- Implement efficient querying strategies
- Use appropriate indexing
- Consider caching for frequently accessed data
- Implement pagination for large result sets

## Common Pitfalls

### Anemic Repositories
- Avoid repositories that are just CRUD operations
- Include domain-specific query methods
- Encapsulate complex business queries

### Leaky Abstractions
- Don't expose data source implementation details
- Avoid returning data source-specific types
- Keep the interface technology-agnostic

### Over-abstraction
- Don't create repositories for every entity
- Consider aggregate boundaries
- Balance abstraction with simplicity

## Related Patterns

- **Unit of Work**: Manages transactions across multiple repositories
- **Data Mapper**: Maps between domain objects and database records
- **Active Record**: Alternative pattern where domain objects handle their own persistence
- **Specification**: Encapsulates query logic in reusable objects