const querystring = require('querystring')

function call(r) {

  const config = createConfigurationFromVariables(r)
  const request = buildRequest(config)

  r.subrequest(request.url, request.query,
    function(reply) {
      if (reply.headersOut["X-Sablier-Session-Status"] == "ready") {
        r.internalRedirect(config.internalRedirect);
      } else {
        r.headersOut["Content-Type"] = reply.headersOut["Content-Type"]
        r.headersOut["Content-Length"] = reply.headersOut["Content-Length"]
        r.return(200, reply.responseBuffer);
      }
    }
  );
}

/**
 * @typedef {Object} SablierConfig
 * @property {string} sablierUrl
 * @property {string} names
 * @property {string} group
 * @property {string} sessionDuration
 * @property {string} internalRedirect
 * @property {string} displayName
 * @property {string} showDetails
 * @property {string} theme
 * @property {string} refreshFrequency
 * @property {string} timeout
 * 
 */

/**
 * 
 * @param {*} headers 
 * @returns {SablierConfig}
 */
function createConfigurationFromVariables(r) {
  return {
    sablierUrl: r.variables.sablierUrl,
    names: r.variables.sablierNames,
    group: r.variables.sablierGroup,
    sessionDuration: r.variables.sablierSessionDuration,
    internalRedirect: r.variables.sablierNginxInternalRedirect,

    displayName:  r.variables.sablierDynamicName,
    showDetails:  r.variables.sablierDynamicShowDetails,
    theme:  r.variables.sablierDynamicTheme,
    refreshFrequency:  r.variables.sablierDynamicRefreshFrequency,

    timeout:  r.variables.sablierBlockingTimeout,
  }
}

/**
 * 
 * @param {SablierConfig} c 
 * @returns 
 */
function buildRequest(c) {
	if (c.timeout == undefined || c.timeout == "") {
		return createDynamicUrl(c)
	} else {
		return createBlockingUrl(c)
	}
}

/**
 * 
 * @param {SablierConfig} config 
 * @returns 
 */
 function createDynamicUrl(config) {
  const url = `${config.sablierUrl}/api/strategies/dynamic`
  const query = {
    session_duration: config.sessionDuration,
    display_name:config.displayName,
    theme: config.theme,
    refresh_frequency: config.refreshFrequency,
    show_details: config.showDetails
  };

  if(config.names) {
    query.names = config.names.split(",").map(name => name.trim())
  } else if(config.group) {
    query.group = config.group
  } else {
    throw new Error('you must specify names or group'); 
  }

	return {url, query: querystring.stringify(query)}
}

/**
 * 
 * @param {SablierConfig} config 
 * @returns 
 */
 function createBlockingUrl(config) {
  const url = `${config.sablierUrl}/api/strategies/blocking`
  const query = { 
    session_duration: config.sessionDuration,
    timeout:config.timeout,
  };

  if(config.names) {
    query.names = config.names.split(",").map(name => name.trim())
  } else if(config.group) {
    query.group = config.group
  } else {
    throw new Error('you must specify names or group'); 
  }

	return {url, query: querystring.stringify(query)} 
}

export default { call };