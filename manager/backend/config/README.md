# Configuration File Documentation

## Configuration File List

- `config.json` - Default configuration file
- `config.dev.json` - Development environment configuration
- `config.prod.json` - Production environment configuration
- `config.example.json` - Configuration file example

## Configuration File Structure

```json
{
  "server": {
    "port": "8080",        // Server port
    "mode": "debug"        // Run mode: debug/release
  },
  "database": {
    "host": "localhost",   // Database host
    "port": "3306",        // Database port
    "username": "root",    // Database username
    "password": "password", // Database password
    "database": "xiaozhi_admin" // Database name
  },
  "jwt": {
    "secret": "your_secret_key", // JWT signing key
    "expire_hour": 24           // Token expiration time (hours)
  }
}
```

## Usage

### 1. Command Line Arguments

```bash
# Use default configuration file
go run main.go

# Specify configuration file
go run main.go -config=config/config.dev.json
go run main.go -c config/config.prod.json
```

### 2. Startup Scripts

**Windows:**
```cmd
start.bat                    # Default configuration
start.bat dev                # Development environment
start.bat prod               # Production environment
start.bat custom my.json     # Custom configuration
start.bat help               # Show help
```

**Linux/Mac:**
```bash
./start.sh                   # Default configuration
./start.sh dev               # Development environment
./start.sh prod              # Production environment
./start.sh custom my.json    # Custom configuration
./start.sh help              # Show help
```

## Environment Configuration Recommendations

### Development Environment (config.dev.json)
- Use debug mode
- Add _dev suffix to database name
- JWT key can use simple string
- Token expiration time can be set longer

### Production Environment (config.prod.json)
- Use release mode
- Use independent production database
- JWT key must use strong password
- Token expiration time should be set shorter
- Database user permissions minimized

## Security Notes

1. **Do not commit production environment configuration files to version control**
2. **JWT key must be kept secret and complex enough**
3. **Database passwords should be changed regularly**
4. **Production environment recommends using environment variables to override sensitive configuration**

## Configuration File Priority

1. Configuration file specified on command line
2. Default configuration file (config.json)

## Troubleshooting

### Configuration file does not exist
```
Error: Unable to open configuration file config/missing.json: no such file or directory
```
**Solution**: Check if configuration file path is correct

### Configuration file format error
```
Error: Failed to parse configuration file config/config.json: invalid character '}' looking for beginning of object key string
```
**Solution**: Check if JSON format is correct, can use JSON validation tools

### Database connection failed
```
Error: Database connection failed: Error 1045: Access denied for user 'root'@'localhost'
```
**Solution**: Check if database configuration information is correct
