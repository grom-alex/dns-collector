import os
import socket
import json
import sys
import unbound


UDP_IP = "192.168.0.15"
UDP_PORT = 5353

sock = None

def send_udp(rtype, qinfo, **kwargs):
    global sock
    if sock is None:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            log_info("sock created lazily")
        except Exception as e:
            log_info("Failed to create UDP socket: " + str(e))
            return

    repinfo = kwargs.get("repinfo")
    if repinfo:
        try:
            client_ip = repinfo.addr
        except:
            client_ip = "unknown"
    else:
        client_ip = "unknown"
        
    if client_ip == UDP_IP:
        return

    domain = qinfo.qname_str
    qtype  = qinfo.qtype_str

    msg = {
        "client_ip": client_ip,
        "domain": domain,
        "qtype": qtype,
        "rtype": rtype,
    }
    #log_info("python: msg " + json.dumps(msg))
    try:
        sock.sendto(json.dumps(msg).encode(), (UDP_IP, UDP_PORT))
        #log_info(f"sending to {UDP_IP}")
    except Exception as e:
        log_info("Failed to send UDP: " + str(e))

def inplace_reply_callback(qinfo, qstate, rep, rcode, edns, opt_list_out,
                           region, **kwargs):
    """
    Function that will be registered as an inplace callback function.
    It will be called when answering with a resolved query.

    :param qinfo: query_info struct;
    :param qstate: module qstate. It contains the available opt_lists; It
                   SHOULD NOT be altered;
    :param rep: reply_info struct;
    :param rcode: return code for the query;
    :param edns: edns_data to be sent to the client side. It SHOULD NOT be
                 altered;
    :param opt_list_out: the list with the EDNS options that will be sent as a
                         reply. It can be populated with EDNS options;
    :param region: region to allocate temporary data. Needs to be used when we
                   want to append a new option to opt_list_out.
    :param **kwargs: Dictionary that may contain parameters added in a future
                     release. Current parameters:
        ``repinfo``: Reply information for a communication point (comm_reply).

    :return: True on success, False on failure.

    """
    send_udp("reply", qinfo, **kwargs)
    return True


def inplace_cache_callback(qinfo, qstate, rep, rcode, edns, opt_list_out, region, **kwargs):
    """
    Function that will be registered as an inplace callback function.
    It will be called when answering from the cache.

    :param qinfo: query_info struct;
    :param qstate: module qstate. None;
    :param rep: reply_info struct;
    :param rcode: return code for the query;
    :param edns: edns_data sent from the client side. The list with the EDNS
                 options is accessible through edns.opt_list. It SHOULD NOT be
                 altered;
    :param opt_list_out: the list with the EDNS options that will be sent as a
                         reply. It can be populated with EDNS options;
    :param region: region to allocate temporary data. Needs to be used when we
                   want to append a new option to opt_list_out.
    :param **kwargs: Dictionary that may contain parameters added in a future
                     release. Current parameters:
        ``repinfo``: Reply information for a communication point (comm_reply).

    :return: True on success, False on failure.

    For demonstration purposes we want to see if EDNS option 65002 is present
    and reply with a new value.

    """
    
    send_udp("cache", qinfo, **kwargs)
    return True

def inplace_query_callback(qinfo, flags, qstate, addr, zone, region, **kwargs):
    """
    Function that will be registered as an inplace callback function.
    It will be called before sending a query to a backend server.

    :param qinfo: query_info struct;
    :param flags: flags of the query;
    :param qstate: module qstate. opt_lists are available here;
    :param addr: struct sockaddr_storage. Address of the backend server;
    :param zone: zone name in binary;
    :param region: region to allocate temporary data. Needs to be used when we
                   want to append a new option to opt_lists.
    :param **kwargs: Dictionary that may contain parameters added in a future
                     release.
    """
    log_info("python: outgoing query to {}@{}, d: {}".format(addr.addr, addr.port, qinfo.qname_str))
    return True

def inplace_local_callback(qinfo, qstate, rep, rcode, edns, opt_list_out, region, **kwargs):
    """
    Function that will be registered as an inplace callback function.
    It will be called when answering from local data.

    :param qinfo: query_info struct;
    :param qstate: module qstate. None;
    :param rep: reply_info struct;
    :param rcode: return code for the query;
    :param edns: edns_data sent from the client side. The list with the
                 EDNS options is accessible through edns.opt_list. It
                 SHOULD NOT be altered;
    :param opt_list_out: the list with the EDNS options that will be sent as a
                         reply. It can be populated with EDNS options;
    :param region: region to allocate temporary data. Needs to be used when we
                   want to append a new option to opt_list_out.
    :param **kwargs: Dictionary that may contain parameters added in a future
                     release. Current parameters:
        ``repinfo``: Reply information for a communication point (comm_reply).

    :return: True on success, False on failure.

    """

    send_udp("reply", qinfo, **kwargs)
    return True

def inform_super(id, qstate, superqstate, qdata):
    return True

def init_standard(id, env):
    log_info("python: inited script {}".format(mod_env['script']))

    # Register the inplace_reply_callback function as an inplace callback
    # function when answering a resolved query.
    if not register_inplace_cb_reply(inplace_reply_callback, env, id):
        return False

    # Register the inplace_cache_callback function as an inplace callback
    # function when answering from cache.
    if not register_inplace_cb_reply_cache(inplace_cache_callback, env, id):
        return False

    # Register the inplace_query_callback function as an inplace callback
    # before sending a query to a backend server.
    #if not register_inplace_cb_query(inplace_query_callback, env, id):
    #    return False

    # Register the inplace_local_callback function as an inplace callback
    # function when answering from local data.
    #if not register_inplace_cb_reply_local(inplace_local_callback, env, id):
    #    return False
    
    return True


def init(id, cfg):
    global sock
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    return True

def deinit(id):
    global sock
    if sock:
        sock.close()
    return True

def operate(id, event, qstate, qdata):
    if (event == MODULE_EVENT_NEW) or (event == MODULE_EVENT_PASS):
        #domain = qstate.qinfo.qname_str
        #log_err(f"pythonmod: [MODULE_EVENT_NEW] Requested dimain {domain}")
        qstate.ext_state[id] = MODULE_WAIT_MODULE 
        return True

    elif event == MODULE_EVENT_MODDONE:
        #domain = qstate.qinfo.qname_str
        #log_err(f"pythonmod: [MODULE_EVENT_MODDONE] Requested dimain {domain}")
        #if (qstate.return_msg):
        #    logDnsMsg(qstate)
        qstate.ext_state[id] = MODULE_FINISHED
        return True

    log_err("pythonmod: Unknown event")
    qstate.ext_state[id] = MODULE_ERROR
    return True
