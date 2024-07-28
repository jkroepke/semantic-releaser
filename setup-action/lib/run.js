"use strict";
// Copyright (c) Jan-Otto Kröpke
// Licensed under the MIT license.
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const os = __importStar(require("os"));
const path = __importStar(require("path"));
const fs = __importStar(require("fs"));
const toolCache = __importStar(require("@actions/tool-cache"));
const core = __importStar(require("@actions/core"));
const http = __importStar(require("@actions/http-client"));
const toolName = 'semantic-releaser';
const githubRepo = 'jkroepke/semantic-releaser';
const stableVersion = '0.0.1';
function getExecutableExtension() {
    if (os.type().match(/^Win/)) {
        return '.exe';
    }
    return '';
}
function getDownloadURL(version) {
    if (version.toLocaleLowerCase().startsWith('v')) {
        version = version.substring(1);
    }
    let arch = os.arch();
    if (arch === 'x64') {
        arch = 'amd64';
    }
    let osType = os.type().toLowerCase();
    if (osType === 'windows_nt') {
        osType = 'windows';
    }
    return `https://github.com/${githubRepo}/releases/download/v${version}/${toolName}_${version}_${osType}_${arch}.tar.gz`;
}
function getStableVersion() {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const httpClient = new http.HttpClient();
            const res = yield httpClient.getJson(`https://github.com/${githubRepo}/releases/latest`);
            return res.result.tag_name;
        }
        catch (e) {
            core.warning(`Cannot get the latest ${toolName} info from https://github.com/${githubRepo}/releases/latest. Error ${e}. Using default version ${stableVersion}.`);
        }
        return stableVersion;
    });
}
const walkSync = function (dir, fileList, fileToFind) {
    const files = fs.readdirSync(dir);
    fileList = fileList || [];
    files.forEach(function (file) {
        if (fs.statSync(path.join(dir, file)).isDirectory()) {
            fileList = walkSync(path.join(dir, file), fileList, fileToFind);
        }
        else {
            core.debug(file);
            if (file === fileToFind) {
                fileList.push(path.join(dir, file));
            }
        }
    });
    return fileList;
};
function downloadBinary(version) {
    return __awaiter(this, void 0, void 0, function* () {
        if (!version) {
            version = yield getStableVersion();
        }
        let cachedToolPath = toolCache.find(toolName, version);
        if (!cachedToolPath) {
            const downloadUrl = getDownloadURL(version);
            let downloadPath;
            try {
                downloadPath = yield toolCache.downloadTool(downloadUrl);
            }
            catch (exception) {
                throw new Error(`Failed to download ${toolName} from location ${downloadUrl}`);
            }
            const extractedFolder = yield toolCache.extractTar(downloadPath);
            fs.chmodSync(extractedFolder, '777');
            cachedToolPath = yield toolCache.cacheFile(extractedFolder + '/' + toolName + getExecutableExtension(), toolName + getExecutableExtension(), toolName, version);
        }
        const binaryPath = findBinary(cachedToolPath);
        if (!binaryPath) {
            throw new Error(`${toolName} executable not found in path ${cachedToolPath}`);
        }
        fs.chmodSync(binaryPath, '777');
        return binaryPath;
    });
}
function findBinary(rootFolder) {
    fs.chmodSync(rootFolder, '777');
    const fileList = [];
    walkSync(rootFolder, fileList, toolName + getExecutableExtension());
    if (!fileList) {
        throw new Error(`${toolName} executable not found in path ${rootFolder}`);
    }
    else {
        return fileList[0];
    }
}
function run() {
    return __awaiter(this, void 0, void 0, function* () {
        let version = core.getInput('version', { 'required': true });
        if (version.toLocaleLowerCase() === 'latest') {
            version = yield getStableVersion();
        }
        else if (!version.toLocaleLowerCase().startsWith('v')) {
            version = 'v' + version;
        }
        const cachedPath = yield downloadBinary(version);
        core.addPath(path.dirname(cachedPath));
        core.setOutput(`${toolName}-path`, cachedPath);
    });
}
run().catch(core.setFailed);
