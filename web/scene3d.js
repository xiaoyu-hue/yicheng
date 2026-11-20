// 一键成城 · 立体沙盘（5.0）
// 页面层三维搭建：只读核心导出的场景 JSON（几何＋配色），不参与生成。
// 依赖 Three.js（three.min.js + OrbitControls，本地打包，离线可用）。
// 坐标换算：城图 (x, y) → 沙盘 (x-640, 高, y-480)；形状先取 (x, -y) 再绕 X 轴 -90° 铺平。
(function (global) {
  'use strict';

  var HEIGHTS = { '民居': 13, '市集': 17, '衙署': 30, '寺庙': 26, '学宫': 22, '园林': 5, '驿站': 16, '水市': 13, '码头': 7 };

  function shapeFrom(THREE, poly) {
    var s = new THREE.Shape();
    poly.forEach(function (p, i) {
      if (i === 0) s.moveTo(p.x, -p.y); else s.lineTo(p.x, -p.y);
    });
    s.closePath();
    return s;
  }

  // ribbon 把一条折线铺成带状平面（河流、街道、护城河用）。glow>0 时带自发光，水面用。
  function ribbon(THREE, pts, width, color, y, closed, glow) {
    var P = closed ? pts.concat([pts[0]]) : pts;
    var pos = [];
    for (var i = 0; i < P.length; i++) {
      var p = P[i];
      var pPrev = P[Math.max(0, i - 1)], pNext = P[Math.min(P.length - 1, i + 1)];
      var dx = pNext.x - pPrev.x, dy = pNext.y - pPrev.y;
      var l = Math.hypot(dx, dy) || 1; dx /= l; dy /= l;
      var nx = -dy * width / 2, ny = dx * width / 2;
      pos.push(p.x - 640 + nx, 0, p.y - 480 + ny, p.x - 640 - nx, 0, p.y - 480 - ny);
    }
    var idx = [];
    for (var j = 0; j + 1 < P.length; j++) {
      var a = j * 2;
      idx.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
    }
    var g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.setIndex(idx);
    g.computeVertexNormals();
    var m = new THREE.Mesh(g, new THREE.MeshLambertMaterial({
      color: color, side: THREE.DoubleSide,
      emissive: glow ? color : 0x000000, emissiveIntensity: glow || 0
    }));
    m.position.y = y;
    return m;
  }

  function buildCityGroup(THREE, S) {
    var C = S.colors;
    function mat(c) { return new THREE.MeshLambertMaterial({ color: c }); }
    var g = new THREE.Group();

    // 纸面大地
    var base = new THREE.Mesh(new THREE.PlaneGeometry(1320, 1020), mat(C.paper));
    base.rotation.x = -Math.PI / 2;
    base.position.y = -0.3;
    base.receiveShadow = true;
    g.add(base);

    // 城外田野
    (S.fields || []).forEach(function (f) {
      var m = new THREE.Mesh(new THREE.ShapeGeometry(shapeFrom(THREE, f.Poly)), mat(f.Tone === 1 ? C.fieldB : C.fieldA));
      m.rotation.x = -Math.PI / 2;
      m.position.set(-640, 0.5, -480);
      g.add(m);
    });

    // 城内地面
    var ground = new THREE.Mesh(new THREE.ShapeGeometry(shapeFrom(THREE, S.boundary)), mat(C.ground));
    ground.rotation.x = -Math.PI / 2;
    ground.position.set(-640, 0.15, -480);
    ground.receiveShadow = true;
    g.add(ground);

    // 水系：城南河、运河、护城河
    if (S.river && S.river.length > 1) g.add(ribbon(THREE, S.river, 56, C.water, 0.8, false, 0.5));
    (S.canals || []).forEach(function (cn) { g.add(ribbon(THREE, cn, 18, C.water, 1.5, false, 0.5)); });
    var moat = S.boundary.map(function (p) {
      return { x: 640 + (p.x - 640) * 1.14, y: 480 + (p.y - 480) * 1.14 };
    });
    g.add(ribbon(THREE, moat, 22, C.water, 0.8, true, 0.5));

    // 街道：环街与主街
    if (S.ring && S.ring.length > 3) g.add(ribbon(THREE, S.ring, 8, C.road, 1.2, true));
    (S.roads || []).forEach(function (r) { g.add(ribbon(THREE, r, 11, C.road, 1.2, false)); });

    // 坊块（按用途定高）＋地标大屋顶
    (S.wards || []).forEach(function (w) {
      var h = HEIGHTS[w.kind] || 7;
      var geo = new THREE.ExtrudeGeometry(shapeFrom(THREE, w.poly), { depth: h, bevelEnabled: false });
      var m = new THREE.Mesh(geo, mat(w.fill));
      m.rotation.x = -Math.PI / 2;
      m.position.set(-640, 0, -480);
      m.castShadow = true;
      m.receiveShadow = true;
      g.add(m);
      if (w.kind === '衙署' || w.kind === '寺庙' || w.kind === '学宫') {
        var cx = 0, cy = 0, minx = 1e9, maxx = -1e9, miny = 1e9, maxy = -1e9;
        w.poly.forEach(function (p) {
          cx += p.x; cy += p.y;
          minx = Math.min(minx, p.x); maxx = Math.max(maxx, p.x);
          miny = Math.min(miny, p.y); maxy = Math.max(maxy, p.y);
        });
        cx /= w.poly.length; cy /= w.poly.length;
        var rw = Math.max(maxx - minx, maxy - miny) * 0.30;
        var roof = new THREE.Mesh(new THREE.ConeGeometry(rw, 22, 4), mat(C.roofBig));
        roof.position.set(cx - 640, h + 11, cy - 480);
        roof.rotation.y = Math.PI / 4;
        roof.scale.z = Math.max(0.5, Math.min(1.6, (maxy - miny) / (maxx - minx || 1)));
        roof.castShadow = true;
        g.add(roof);
      }
    });

    // 屋舍：小盒＋四坡小顶，落在所在坊块的顶面上（不埋进坊块里）
    function wardHeightAt(x, y) {
      for (var wi = 0; wi < (S.wards || []).length; wi++) {
        var p = S.wards[wi].poly, inside = false;
        for (var i = 0, j = p.length - 1; i < p.length; j = i++) {
          var xi = p[i].x, yi = p[i].y, xj = p[j].x, yj = p[j].y;
          if (((yi > y) !== (yj > y)) && (x < (xj - xi) * (y - yi) / (yj - yi) + xi)) inside = !inside;
        }
        if (inside) return HEIGHTS[S.wards[wi].kind] || 7;
      }
      return 0;
    }
    (S.buildings || []).forEach(function (bd) {
      var baseY = wardHeightAt(bd.CX, bd.CY);
      var box = new THREE.Mesh(new THREE.BoxGeometry(bd.HW * 2, 8, bd.HH * 2), mat(C.building));
      box.position.set(bd.CX - 640, baseY + 4, bd.CY - 480);
      box.rotation.y = -bd.Ang;
      box.castShadow = true;
      g.add(box);
      var roof = new THREE.Mesh(new THREE.ConeGeometry(bd.HW * 1.45, 5.6, 4), mat(C.roof));
      roof.position.set(bd.CX - 640, baseY + 10.8, bd.CY - 480);
      roof.rotation.y = Math.PI / 4 - bd.Ang;
      roof.scale.z = bd.HH / bd.HW;
      g.add(roof);
    });

    // 城墙＋门楼
    (S.wallEdges || []).forEach(function (e) {
      var a = e[0], b = e[1];
      var len = Math.hypot(b.x - a.x, b.y - a.y);
      var wall = new THREE.Mesh(new THREE.BoxGeometry(len + 2, 30, 8), mat(C.wall));
      wall.position.set((a.x + b.x) / 2 - 640, 15, (a.y + b.y) / 2 - 480);
      wall.rotation.y = -Math.atan2(b.y - a.y, b.x - a.x);
      wall.castShadow = true;
      wall.receiveShadow = true;
      g.add(wall);
    });
    (S.gates || []).forEach(function (gt) {
      var tower = new THREE.Mesh(new THREE.BoxGeometry(20, 40, 20), mat(C.wallTop));
      tower.position.set(gt.mid.x - 640, 20, gt.mid.y - 480);
      tower.castShadow = true;
      g.add(tower);
      var roof = new THREE.Mesh(new THREE.ConeGeometry(18, 11, 4), mat(C.roofBig));
      roof.position.set(gt.mid.x - 640, 45.5, gt.mid.y - 480);
      roof.rotation.y = Math.PI / 4;
      g.add(roof);
    });

    return g;
  }

  // createViewer 在 container 里建渲染器；getScene() 返回 Promise<场景 JSON>。
  function createViewer(THREE, container, getScene) {
    var renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(global.devicePixelRatio || 1, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(renderer.domElement);

    var scene = new THREE.Scene();
    var camera = new THREE.PerspectiveCamera(42, 16 / 10, 10, 3200);
    camera.position.set(110, 400, 560);

    var controls = new THREE.OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.06;
    controls.maxPolarAngle = 1.35;
    controls.minDistance = 260;
    controls.maxDistance = 1400;
    controls.autoRotate = true;
    controls.autoRotateSpeed = 1.2;
    controls.addEventListener('start', function () { controls.autoRotate = false; });

    var hemi = new THREE.HemisphereLight(0xfff4e0, 0x8a7a5a, 0.72);
    scene.add(hemi);
    var sun = new THREE.DirectionalLight(0xffe8c0, 0.62);
    sun.position.set(420, 620, 240);
    sun.castShadow = true;
    sun.shadow.mapSize.set(1024, 1024);
    sun.shadow.camera.left = -580; sun.shadow.camera.right = 580;
    sun.shadow.camera.top = 580; sun.shadow.camera.bottom = -580;
    scene.add(sun);

    var group = null;
    var visible = false;

    function resize() {
      var w = container.clientWidth || 800;
      var h = Math.round(w * 0.62);
      renderer.setSize(w, h);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    }
    if (global.ResizeObserver) new ResizeObserver(resize).observe(container);
    resize();
    if (global.requestAnimationFrame) requestAnimationFrame(resize);

    (function loop() {
      requestAnimationFrame(loop);
      if (visible) {
        controls.update();
        renderer.render(scene, camera);
      }
    })();

    return {
      setVisible: function (v) { visible = v; if (v) resize(); },
      refresh: function () {
        return getScene().then(function (S) {
          if (S.region) return 'region';
          scene.background = new THREE.Color(S.colors.paper);
          scene.fog = new THREE.Fog(S.colors.paper, 1200, 2600);
          if (S.theme === 'night') { hemi.intensity = 0.42; sun.intensity = 0.38; }
          else { hemi.intensity = 0.72; sun.intensity = 0.62; }
          if (group) scene.remove(group);
          group = buildCityGroup(THREE, S);
          scene.add(group);
          resize();
          return 'ok';
        });
      }
    };
  }

  global.YC3D = { buildCityGroup: buildCityGroup, createViewer: createViewer, HEIGHTS: HEIGHTS };
})(typeof window !== 'undefined' ? window : globalThis);
