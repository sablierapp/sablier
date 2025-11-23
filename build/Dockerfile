FROM scratch

ARG TARGETPLATFORM

# Copy binary and config
COPY --chmod=0755 $TARGETPLATFORM/sablier /bin/sablier
COPY --chmod=0644 build/sablier.yaml /etc/sablier/sablier.yaml

EXPOSE 10000

VOLUME [ "/etc/sablier/state.json", "/etc/sablier/themes" ]

ENTRYPOINT [ "/bin/sablier" ]
CMD [ "--configFile=/etc/sablier/sablier.yaml", "start" ]
