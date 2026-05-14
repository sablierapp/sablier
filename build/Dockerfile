FROM scratch

ARG TARGETPLATFORM

# Copy binary and config
COPY --chmod=0755 $TARGETPLATFORM/sablier /bin/sablier

EXPOSE 10000

ENTRYPOINT [ "/bin/sablier" ]
CMD [ "start" ]
