// Copyright 2013 Chris McGee <sirnewton_01@yahoo.ca>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
define(['orion/xhr', 'orion/plugin', 'orion/form'], function (xhr, PluginProvider, form) {
    var headers = {
        name: "GoDev Oracle IDE Bundle",
        version: "1.0",
        description: "GoDev Oracle Integration (references, implements, callers, peers)"
    };
    
    var provider = new PluginProvider(headers);
    
	provider.registerService(
		"orion.edit.command", 
		{
			run: function(selectedText, text, selection, resource) {				
				return {uriTemplate: "/godev-oracle/referrers.html?resource="+resource+"&pos="+selection.start, width: "400px", height: "400px"};
			}
		},
		{
			name: "References",
			id: "go.referrers",
			tooltip: "Find code that references this item (F4)",
			key: [115],
			contentType: ["text/x-go"]
		});
        
	provider.registerService(
		"orion.edit.command", 
		{
			run: function(selectedText, text, selection, resource) {				
				return {uriTemplate: "/godev-oracle/implements.html?resource="+resource+"&pos="+selection.start, width: "400px", height: "400px"};
			}
		},
		{
			name: "Implements",
			id: "go.implements",
			tooltip: "Find types that implement an interface (Ctrl-M)",
			key: ["M", true],
			contentType: ["text/x-go"]
		});
		
	provider.registerService(
		"orion.edit.command", 
		{
			run: function(selectedText, text, selection, resource) {				
				return {uriTemplate: "/godev-oracle/callers.html?resource="+resource+"&pos="+selection.start, width: "400px", height: "400px"};
			}
		},
		{
			name: "Callers",
			id: "go.callers",
			tooltip: "Find callers of this function (Ctrl-Shift-H)",
			key: ["H", true, true],
			contentType: ["text/x-go"]
		});
		
	provider.registerService(
		"orion.edit.command", 
		{
			run: function(selectedText, text, selection, resource) {				
				return {uriTemplate: "/godev-oracle/peers.html?resource="+resource+"&pos="+selection.start, width: "400px", height: "400px"};
			}
		},
		{
			name: "Peers",
			id: "go.peers",
			tooltip: "Find other locations that allocate/send/receive on the channel",
			contentType: ["text/x-go"]
		});
		
	provider.connect();
});