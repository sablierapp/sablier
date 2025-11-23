FROM scratch

ARG TARGETPLATFORM

# Run as non-root user
USER 65532:65532

# Add non-root user
COPY --chmod=0755 --chown=65532:65532 $TARGETPLATFORM/sablier /bin/sablier
COPY --chmod=0644 --chown=65532:65532 build/sablier.yaml /etc/sablier/sablier.yaml

EXPOSE 10000

ENTRYPOINT [ "/bin/sablier" ]
CMD [ "--configFile=/etc/sablier/sablier.yaml", "start" ]
