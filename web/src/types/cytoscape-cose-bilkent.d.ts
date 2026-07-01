declare module 'cytoscape-cose-bilkent' {
  // The library exports a single ext function (the register callback).
  // cytoscape.use() accepts cytoscape.Ext; we keep the type loose here to
  // match the runtime signature without pulling in a stronger dependency.
  const ext: (cytoscape: unknown) => void;
  export default ext;
}
