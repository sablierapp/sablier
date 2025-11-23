FROM scratch

ARG TARGETPLATFORM

# Add non-root user
COPY --chmod=0755 $TARGETPLATFORM/sablier /bin/sablier
COPY --chmod=0644 build/sablier.yaml /etc/sablier/sablier.yaml

# Run as non-root user
USER 65532:65532

EXPOSE 10000

ENTRYPOINT [ "/bin/sablier" ]
CMD [ "--configFile=/etc/sablier/sablier.yaml", "start" ]
