FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/uploadstream .

# Create data directory for file storage
RUN mkdir -p /app/data/files

# Expose gRPC port and metrics port
EXPOSE 50051 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9090/health || exit 1

# Run as non-root user
RUN adduser -D -u 1000 appuser
RUN chown -R appuser:appuser /app
USER appuser

ENTRYPOINT ["./uploadstream"]