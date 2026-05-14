FROM scratch

ARG TARGETPLATFORM

# Copy binary and config
COPY --chmod=0755 $TARGETPLATFORM/sablier /bin/sablier

# Default timezone can still be overridden at runtime with -e TZ=...
ENV TZ=UTC

EXPOSE 10000

ENTRYPOINT [ "/bin/sablier" ]
CMD [ "start" ]
